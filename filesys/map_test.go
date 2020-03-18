package filesys

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

var FS = Map(
	map[string][]byte{
		"bar/baz/file3": []byte("file3 contents"),
		"bar/file4":     []byte("file4 contents"),
		"bar/link1":     []byte("foo/file2"),
		"foo/file2":     []byte("file2 contents"),
		"foo/link2":     []byte("bar/baz"),
		"file1":         []byte("file1 contents"),
	},
	[]string{
		"bar/link1",
		"foo/link2",
	},
)

func TestOpen(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"bar/baz/file3", "file3 contents"},
		{"bar/link1", "foo/file2"},
		{"foo/file2", "file2 contents"},
		{"foo/link2", "bar/baz"},
		{"file1", "file1 contents"},
	}
	for _, tt := range tests {
		f, err := FS.Open(tt.path)
		if err != nil {
			t.Errorf("Open(%q) = %v", tt.path, err)
			continue
		}
		b, _ := ioutil.ReadAll(f)
		if string(b) != tt.want {
			t.Errorf("want %s; got %s", tt.want, string(b))
		}
	}

	if _, err := FS.Open("foo/bogus"); err != os.ErrNotExist {
		t.Errorf("want os.ErrNotExist; got %v", err)
	}
}

func TestLstat(t *testing.T) {
	tests := []struct {
		path string
		want os.FileInfo
	}{
		{"file1", fileInfo("file1", len("file1 contents"), false)},
		{"bar/link1", fileInfo("link1", len("foo/file2"), true)},
		{"foo/link2", fileInfo("link2", len("bar/baz"), true)},
	}
	for _, tt := range tests {
		i, err := FS.Lstat(tt.path)
		if err != nil {
			t.Errorf("Lstat(%q) = %v", tt.path, err)
			continue
		}
		if !reflect.DeepEqual(tt.want, i) {
			t.Errorf("want %#v; got %#v", tt.want, i)
		}
	}

	if _, err := FS.Lstat("foo/bogus"); err != os.ErrNotExist {
		t.Errorf("want os.ErrNotExist; got %v", err)
	}
}

func TestReadlink(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"bar/link1", "foo/file2"},
		{"foo/link2", "bar/baz"},
	}
	for _, tt := range tests {
		got, err := FS.Readlink(tt.path)
		if err != nil {
			t.Errorf("Readlink(%q) = %v", tt.path, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Readlink(%q) = %q; want %q", tt.path, got, tt.want)
		}
	}

	_, err := FS.Readlink("foo/file2")
	if _, ok := err.(*os.PathError); err == nil || !ok {
		t.Errorf("want os.PathError; got %v", err)
	}
}

func TestReaddirnames(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"", []string{"bar", "file1", "foo"}},
		{".", []string{"bar", "file1", "foo"}},
		{"/", []string{"bar", "file1", "foo"}},
		{"bar", []string{"baz", "file4", "link1"}},
		{"bar/baz", []string{"file3"}},
		{"./bar/baz/../baz/", []string{"file3"}},
		{"foo", []string{"file2", "link2"}},
	}
	for _, tt := range tests {
		got, err := FS.Readdirnames(tt.path)
		if err != nil {
			t.Errorf("Readdirnames(%q) = %v", tt.path, err)
			continue
		}
		if !reflect.DeepEqual(tt.want, got) {
			t.Errorf("Readdirnames(%q) = %v; want %v", tt.path, got, tt.want)
		}
	}

	if _, err := FS.Readdirnames("file1"); err != os.ErrNotExist {
		t.Errorf("want os.ErrNotExist; got %v", err)
	}
	if _, err := FS.Readdirnames(".."); err != os.ErrNotExist {
		t.Errorf("want os.ErrNotExist; got %v", err)
	}
}
