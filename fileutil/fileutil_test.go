package fileutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExistFileOrDir(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if !ExistFileOrDir(tmpDir) {
		t.Fatalf("expected %q 'exist'", tmpDir)
	}

	f, err := ioutil.TempFile(os.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if ok := ExistFileOrDir(f.Name()); !ok {
		t.Fatalf("%s does not exist", f.Name())
	}

	os.Remove(f.Name())
	if ok := ExistFileOrDir(f.Name()); ok {
		t.Fatalf("%s should not exist", f.Name())
	}
}

func TestMkdirAllEmpty(t *testing.T) {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "foo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	tmpdir2 := filepath.Join(tmpdir, "testdir")
	if err = MkdirAllEmpty(tmpdir2); err != nil {
		t.Fatal(err)
	}

	if err = ioutil.WriteFile(filepath.Join(tmpdir2, "text.txt"), []byte("test text"), PrivateFileMode); err != nil {
		t.Fatal(err)
	}

	if err = MkdirAllEmpty(tmpdir2); err == nil || !strings.Contains(err.Error(), "to be empty, got") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestDirWritable(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err = DirWritable(tmpDir); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(tmpDir, 0444); err != nil { // READ-ONLY
		t.Fatal(err)
	}
	if err = DirWritable(tmpDir); err == nil {
		t.Fatal("expected error")
	}
}

func TestReadDir(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	files := []string{"x", "a", "b"}
	for _, f := range files {
		var file *os.File
		file, err = os.Create(filepath.Join(tmpDir, f))
		if err != nil {
			t.Fatal(err)
		}
		if err = file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	fs, err := ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fs, []string{"a", "b", "x"}) {
		t.Fatalf("unexpected slice %v", fs)
	}
}
