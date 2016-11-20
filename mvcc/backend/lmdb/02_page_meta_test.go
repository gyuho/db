package lmdb

import (
	"reflect"
	"sort"
	"testing"
	"testing/quick"
)

// (bolt.TestPgids_merge, TestPgids_merge_quick)
func Test_pgids_merge(t *testing.T) {
	a := pgids{4, 5, 6, 10, 11, 12, 13, 27}
	b := pgids{1, 3, 8, 9, 25, 30}
	c := a.merge(b)
	if !reflect.DeepEqual(c, pgids{1, 3, 4, 5, 6, 8, 9, 10, 11, 12, 13, 25, 27, 30}) {
		t.Fatalf("mismatch: %v", c)
	}

	a = pgids{4, 5, 6, 10, 11, 12, 13, 27, 35, 36}
	b = pgids{8, 9, 25, 30}
	c = a.merge(b)
	if !reflect.DeepEqual(c, pgids{4, 5, 6, 8, 9, 10, 11, 12, 13, 25, 27, 30, 35, 36}) {
		t.Fatalf("mismatch: %v", c)
	}

	if err := quick.Check(func(a, b pgids) bool {
		sort.Sort(a)
		sort.Sort(b)
		got := a.merge(b)

		exp := append(a, b...)
		sort.Sort(exp)

		if !reflect.DeepEqual(exp, got) {
			t.Errorf("\nexp=%+v\ngot=%+v\n", exp, got)
			return false
		}
		return true
	}, nil); err != nil {
		t.Fatal(err)
	}
}

func Test_page_String(t *testing.T) {
	pg := page{flags: freelistPageFlag}
	if ps := pg.String(); ps != "freelist" {
		t.Fatalf("page expected 'freelist', got %q", ps)
	}

	pg.flags |= metaPageFlag
	if ps := pg.String(); ps != "meta" {
		t.Fatalf("page expected 'meta', got %q", ps)
	}

	pg.flags |= leafPageFlag
	if ps := pg.String(); ps != "leaf" {
		t.Fatalf("page expected 'leaf', got %q", ps)
	}

	pg.flags |= branchPageFlag
	if ps := pg.String(); ps != "branch" {
		t.Fatalf("page expected 'branch', got %q", ps)
	}

	pg.flags = 20000
	if ps := pg.String(); ps != "unknown<0x4E20>" {
		t.Fatalf("page expected 'unknown<0x4E20>', got %q", ps)
	}
}

func Test_page_hexdump(*testing.T) {
	(&page{id: 256}).hexdump(16)
}
