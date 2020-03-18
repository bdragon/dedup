// Package filesys provides an abstraction for working with file systems,
// mainly to facilitate testing.
package filesys

import (
	"io"
	"os"
	"sort"
)

// FileSystem provides the interface for operations over a file system.
type FileSystem interface {
	Open(path string) (File, error)
	Lstat(path string) (os.FileInfo, error)
	Readlink(path string) (string, error)
	Readdirnames(path string) ([]string, error)
}

// File provides the interface implemented by values returned from a file
// system's Open method.
type File interface {
	io.Reader
	io.Seeker
	io.Closer
}

// OS returns a FileSystem for working with os files.
func OS() FileSystem {
	return osFS{}
}

type osFS struct{}

func (osFS) Open(pth string) (File, error) { return os.Open(pth) }

func (osFS) Lstat(pth string) (os.FileInfo, error) { return os.Lstat(pth) }

func (osFS) Readlink(pth string) (string, error) { return os.Readlink(pth) }

func (osFS) Readdirnames(pth string) (names []string, err error) {
	f, err := os.Open(pth)
	if err != nil {
		return
	}
	names, err = f.Readdirnames(0)
	_ = f.Close()
	if err != nil {
		return
	}
	sort.Strings(names)
	return
}
