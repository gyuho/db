package lmdb

import (
	"fmt"
	"sort"
	"unsafe"
)

// freelist is a list of all pages available for allocation.
//
// (bold.freelist)
type freelist struct {
	// all free, available page ids
	ids []pgid

	// freed but still-in-use by open transactions
	pending map[txid][]pgid

	// all free, pending page ids
	cache map[pgid]bool
}

func newFreelist() *freelist {
	return &freelist{
		pending: make(map[txid][]pgid),
		cache:   make(map[pgid]bool),
	}
}

// (bolt.freelist.free_count)
func (f *freelist) freeN() int { return len(f.ids) }

// (bolt.freelist.pending_count)
func (f *freelist) pendingN() (count int) {
	for _, li := range f.pending {
		count += len(li)
	}
	return
}

// (bolt.freelist.count)
func (f *freelist) count() int { return f.freeN() + f.pendingN() }

// (bolt.freelist.size)
func (f *freelist) size() int {
	return pageHeaderSize + (int(unsafe.Sizeof(pgid(0))) * f.count())
}

// (bolt.freelist.all)
func (f *freelist) all() []pgid {
	m := make(pgids, 0)
	for _, li := range f.pending {
		m = append(m, li...)
	}
	sort.Sort(m)
	return pgids(f.ids).merge(m)
}

// allocate returns the starting page id of a contiguous list of pages of the given size.
// If a contiguous block cannot be found, returns 0.
//
// (bolt.freelist.allocate)
func (f *freelist) allocate(n int) pgid {
	if len(f.ids) == 0 {
		return 0
	}

	var start, prev pgid
	for i, cur := range f.ids {
		if cur <= 1 {
			panic(fmt.Errorf("invalid page allocation: %d", cur))
		}

		// reset initial page id if not contiguous yet
		if prev == 0 || cur-prev != 1 {
			start = cur
		}

		// found a contiguous block, remove and return it
		if (cur-start)+1 == pgid(n) {
			if (i + 1) == n {
				f.ids = f.ids[i+1:]
			} else {
				copy(f.ids[i-n+1:], f.ids[i+1:])
				f.ids = f.ids[:len(f.ids)-n]
			}

			// remove from cache
			for j := pgid(0); j < pgid(n); j++ {
				delete(f.cache, start+1)
			}

			return start
		}

		prev = cur
	}

	return 0
}

// free releases a page and its overflow for a given transaction id.
// If the page is already freed, then panic.
//
// (bolt.freelist.free)
func (f *freelist) free(txid txid, p *page) {
	if p.id < 1 {
		panic(fmt.Errorf("cannot free page 0 or 1; got %d", p.id))
	}

	var idsToFree = f.pending[txid]
	for id := p.id; id <= p.id+pgid(p.overflow); id++ {
		if f.cache[id] {
			panic(fmt.Errorf("page %d is already freed", id))
		}

		idsToFree = append(idsToFree, id)
		f.cache[id] = true
	}

	f.pending[txid] = idsToFree
}

// release moves all page ids for a transaction id (or older) in pending to freelist.
//
// (bolt.freelist.release)
func (f *freelist) release(txid txid) {
	var m pgids
	for tid, ids := range f.pending {
		if tid <= txid {
			m = append(m, ids...)
			delete(f.pending, tid)
		}
	}
	sort.Sort(m)
	f.ids = pgids(f.ids).merge(m)
}

// rollback removes the pages for the given pending tx.
//
// (bolt.freelist.rollback)
func (f *freelist) rollback(txid txid) {
	for _, id := range f.pending[txid] {
		delete(f.cache, id)
	}
	delete(f.pending, txid)
}

// (bolt.freelist.freed)
func (f *freelist) cached(pgid pgid) bool {
	return f.cache[pgid]
}

// (bolt.freelist.reindex)
func (f *freelist) reindex() {
	f.cache = make(map[pgid]bool, len(f.ids))
	for _, id := range f.ids {
		f.cache[id] = true
	}
	for _, pids := range f.pending {
		for _, pid := range pids {
			f.cache[pid] = true
		}
	}
}

// max uint16 value (64k)
const maxUint16 = 0xFFFF

// read initialzes the freelist from a page.
//
// (bolt.freelist.read)
func (f *freelist) read(p *page) {
	idx, count := 0, int(p.count)
	if count == maxUint16 { // max uint16 value (64k), considered overflow
		idx = 1
		count = int(((*[maxAllocSize]pgid)(unsafe.Pointer(&p.ptr)))[0])
	}

	if count == 0 {
		f.ids = nil
	} else {
		ids := ((*[maxAllocSize]pgid)(unsafe.Pointer(&p.ptr)))[idx:count]
		f.ids = make([]pgid, len(ids))
		copy(f.ids, ids)
		sort.Sort(pgids(f.ids))
	}

	f.reindex()
}

// write writes all the pages ids to the freelist page.
// All free and pending page ids are to be stored on disk since
// when a program crashes, all pending ids will become free.
//
// (bolt.freelist.write)
func (f *freelist) write(p *page) error {
	ids := f.all()
	p.flags |= freelistPageFlag // update header flag

	if len(ids) == 0 {
		p.count = uint16(0)
	} else if len(ids) < maxUint16 {
		p.count = uint16(len(ids))
		copy(((*[maxAllocSize]pgid)(unsafe.Pointer(&p.ptr)))[:], ids)
	} else {
		p.count = maxUint16
		((*[maxAllocSize]pgid)(unsafe.Pointer(&p.ptr)))[0] = pgid(len(ids))
		copy(((*[maxAllocSize]pgid)(unsafe.Pointer(&p.ptr)))[1:], ids)
	}

	return nil
}

// reload reads the freelist from a page and filters out the pending items.
//
// (bolt.freelist.reload)
func (f *freelist) reload(p *page) {
	f.read(p)

	pcache := make(map[pgid]bool)
	for _, pids := range f.pending {
		for _, pid := range pids {
			pcache[pid] = true
		}
	}

	var a []pgid
	for _, id := range f.ids {
		if !pcache[id] {
			a = append(a, id)
		}
	}
	f.ids = a

	f.reindex()
}
