package filesys

import (
	"bytes"
	"io"
	"os"
	"path"
	"sort"
	"syscall"
	"time"
)

// Map returns a FileSystem for m, wherein keys are file paths and values
// are file contents. File paths should not contain a leading slash. If links
// is not nil, it will be used to simulate symbolic links: for each key in m
// that is also in links, its value in m is treated as the link target.
func Map(m map[string][]byte, links []string) FileSystem {
	lm := make(map[string]interface{})
	for _, link := range links {
		lm[link] = nil
	}
	return &mapFS{m, lm}
}

type mapFS struct {
	files map[string][]byte
	links map[string]interface{} // Symbolic link lookup; values ignored.
}

func (fs *mapFS) Open(pth string) (File, error) {
	b, exist := fs.files[pth]
	if !exist {
		return nil, os.ErrNotExist
	}
	return nopCloser{bytes.NewReader(b)}, nil
}

func (fs *mapFS) Lstat(pth string) (os.FileInfo, error) {
	_, link := fs.links[pth]
	b, exist := fs.files[pth]
	if exist {
		return fileInfo(pth, len(b), link), nil
	}
	names, _ := fs.Readdirnames(pth)
	if len(names) > 0 {
		return dirInfo(pth, link), nil
	}
	return nil, os.ErrNotExist
}

func (fs *mapFS) Readlink(pth string) (string, error) {
	_, link := fs.links[pth]
	b, exist := fs.files[pth]
	if exist && link {
		return string(b), nil
	}
	return "", &os.PathError{Op: "readlink", Path: pth, Err: syscall.EINVAL}
}

// Readdirnames reports the names of files contained by the directory at pth.
// To read the top-level directory, specify an empty string.
func (fs *mapFS) Readdirnames(pth string) (names []string, err error) {
	pth = path.Clean(pth)
	if pth == "" || pth == "/" {
		pth = "."
	}
	seen := make(map[string]bool)
	for p := range fs.files {
		dir := path.Dir(p)
		file := true
		var lastBase string
		for {
			if dir == pth {
				base := lastBase
				if file {
					base = path.Base(p)
				}
				seen[base] = true
			}
			if dir == "." {
				break
			} else {
				file = false
				lastBase = path.Base(dir)
				dir = path.Dir(dir)
			}
		}
	}
	if len(seen) > 0 {
		for name := range seen {
			names = append(names, name)
		}
		sort.Strings(names)
	} else {
		err = os.ErrNotExist
	}
	return
}

func fileInfo(pth string, size int, link bool) os.FileInfo {
	var mode os.FileMode
	if link {
		mode = os.ModeSymlink
	}
	return &info{name: path.Base(pth), size: size, mode: mode}
}

func dirInfo(pth string, link bool) os.FileInfo {
	var mode os.FileMode
	if link {
		mode = os.ModeSymlink
	}
	return &info{name: path.Base(pth), dir: true, mode: mode}
}

// info implements os.FileInfo.
type info struct {
	name string
	size int
	mode os.FileMode
	dir  bool
}

var _ os.FileInfo = (*info)(nil)

func (i *info) Name() string       { return i.name }
func (i *info) Size() int64        { return int64(i.size) }
func (i *info) Mode() os.FileMode  { return i.mode }
func (i *info) ModTime() time.Time { return time.Time{} }
func (i *info) IsDir() bool        { return i.dir }
func (i *info) Sys() interface{}   { return nil }

type nopCloser struct {
	io.ReadSeeker
}

func (c nopCloser) Close() error { return nil }
