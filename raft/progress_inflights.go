package raft

// inflights represents the sliding window of inflight messages
// to this follower. The buffer in inflights contains the last log
// entries of each message.
//
// When it's full, no more messages should be sent to this follower.
// Whenever leader sends out a message to this follower, the index
// of the last entry in the message should be added to inflights.
//
// Each inflight message might contains more than one entry.
// And this is limited Config.MaxEntryPerMsg and Config.MaxSizePerMsg,
// in order to effectively limit the number of inflight messages
// and the bandwidth that each Progress can use.
//
// When the leader receives the response from this follower, it should
// free the previous inflight messages by calling inflights.freeTo with
// the index of the last received entry.
//
// For example, leader sends 10 entries, each of which is 1MB.
// And Config.MaxEntryPerMsg is 5 and Config.MaxSizePerMsg is 5MB.
// Then the flow control will send entries with [1,2,3,4,5], [6,7,8,9,10].
// Then we will have [5, 10] in inflights. And once the follower responds
// with index 10, leader can free 5 and 10 in inflights.
//
// (etcd raft.inflights)
type inflights struct {
	// buffer contains the last entry indexes of each message.
	//
	// (etcd raft.inflights.buffer)
	buffer []uint64

	// bufferSize is the size of the buffer.
	//
	// (etcd raft.inflights.size)
	bufferSize int

	// bufferStart is the starting index in the buffer.
	//
	// (etcd raft.inflights.start)
	bufferStart int

	// bufferCount is the number of inflights in the buffer.
	//
	// (etcd raft.inflights.count)
	bufferCount int
}

// (etcd raft.inflights.newInflights)
func newInflights(size int) *inflights {
	return &inflights{
		// to avoid preallocation
		// buffer: make([]uint64, size),

		bufferSize:  size,
		bufferStart: 0,
		bufferCount: 0,
	}
}

// (etcd raft.inflights.full)
func (ins *inflights) full() bool {
	return ins.bufferCount == ins.bufferSize
}

// grow the inflight buffer by doubling up to inflights.size.
// We grow on demand instead of preallocating, to handle systems
// which have thousands of Raft groups per process.
//
// (etcd raft.inflights.growBuf)
func (ins *inflights) growBuffer() {
	newSize := ins.bufferSize * 2
	if newSize == 0 {
		newSize = 1
	} else if newSize > ins.bufferSize {
		newSize = ins.bufferSize
	}
	newBuffer := make([]uint64, newSize)
	copy(newBuffer, ins.buffer)
	ins.buffer = newBuffer
}

// inflight must be incremental.
//
// (etcd raft.inflights.add)
func (ins *inflights) add(inflight uint64) {
	if ins.full() {
		raftLogger.Panicf("cannot add inflight '%d'' into a full inflights", inflight)
	}

	next := ins.bufferStart + ins.bufferCount
	next = next % ins.bufferSize // rotate
	if next >= len(ins.buffer) {
		ins.growBuffer()
	}

	ins.buffer[next] = inflight
	ins.bufferCount++
}

// freeAll frees all inflights.
//
// (etcd raft.inflights.reset)
func (ins *inflights) freeAll() {
	ins.bufferStart = 0
	ins.bufferCount = 0
}

// freeTo frees inflight messages where index <= 'to'.
//
// (etcd raft.inflights.freeTo)
func (ins *inflights) freeTo(to uint64) {
	if ins.bufferCount == 0 || ins.buffer[ins.bufferStart] > to {
		return
	}

	var (
		cnt   int
		start = ins.bufferStart
	)
	for cnt = 0; cnt < ins.bufferCount; cnt++ {
		if ins.buffer[start] > to {
			// found the first larger inflight
			break
		}

		start++
		start = start % ins.bufferSize
	}

	// free 'cnt' inflights and set new start index
	ins.bufferCount -= cnt
	ins.bufferStart = start

	if ins.bufferCount == 0 {
		// inflights is empty, reset the start index so that we don't grow the
		// buffer unnecessarily.
		ins.bufferStart = 0
	}
}

// (etcd raft.inflights.freeFirstOne)
func (ins *inflights) freeFirstOne() {
	ins.freeTo(ins.buffer[ins.bufferStart])
}
