package lmdb

import "testing"

func TestPageString(t *testing.T) {
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
}
