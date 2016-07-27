package types

import (
	"reflect"
	"sort"
	"testing"
)

func TestUint64Slice(t *testing.T) {
	g := Uint64Slice{10, 500, 5, 1, 100, 25}
	w := Uint64Slice{1, 5, 10, 25, 100, 500}
	sort.Sort(g)

	if !reflect.DeepEqual(g, w) {
		t.Fatalf("slice expected %+v, got %+v", w, g)
	}
}
