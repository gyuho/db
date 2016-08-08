package raftsnap

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"
)

// (etcd snap.TestSnapNames)
func Test_Snapshotter_getSnapNames(t *testing.T) {
	dir := path.Join(os.TempDir(), "snapshot")
	err := os.Mkdir(dir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	for i := 1; i <= 5; i++ {
		var f *os.File
		if f, err = os.Create(filepath.Join(dir, fmt.Sprintf("%d.snap", i))); err != nil {
			t.Fatal(err)
		} else {
			f.Close()
		}
	}
	ss := New(dir)

	names, err := getSnapNames(ss.dir)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(names) != 5 {
		t.Fatalf("len = %d, want 10", len(names))
	}
	w := []string{"5.snap", "4.snap", "3.snap", "2.snap", "1.snap"}
	if !reflect.DeepEqual(names, w) {
		t.Fatalf("names = %v, want %v", names, w)
	}
}
