// Package dedup exposes primitives for detecting files with duplicate checksums
// from a list of file paths.
package dedup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/bdragon/dedup/filesys"
)

// Options groups configuration options for Filter and FilterDir.
type Options struct {
	FollowSymlinks bool            // Follow symbolic links.
	Recursive      bool            // Recurse if reading from a directory.
	ExitOnError    bool            // Stop if an error occurs.
	ExitOnDup      bool            // Stop if a file with a previously-seen checksum is found.
	Cancel         <-chan struct{} // Close to signal cancellation.
	UniqWriter     io.Writer       // Write paths of files with previously-unseen checksums.
	DupWriter      io.Writer       // Write paths of files with previously-seen checksums.
	ErrWriter      io.Writer       // Write errors.

	fs filesys.FileSystem
}

// Errors implements the error interface for a slice of errors.
type Errors []error

func (el Errors) Error() string {
	s := make([]string, len(el))
	for i, err := range el {
		s[i] = err.Error()
	}
	return strings.Join(s, "\n")
}

// Filter reads newline-delimited file paths from r, evaluates each file in
// search of duplicate checksums, and returns a *Sums and any error(s) that
// may have occurred during evaluation. If err is non-nil, its type will be
// Errors.
func Filter(r io.Reader, opts *Options) (*Sums, error) {
	if opts.fs == nil {
		opts.fs = filesys.OS()
	}
	f := newChanFilter(readLines(r), maxProcs, opts)
	return run(f, opts)
}

// FilterDir is like Filter except it reads file paths from the directory
// located at path.
func FilterDir(path string, opts *Options) (*Sums, error) {
	if opts.fs == nil {
		opts.fs = filesys.OS()
	}
	f := newDirFilter(path, opts)
	return run(f, opts)
}

// run starts and monitors the specified filter and returns f.Sums() and any
// error(s) that may have occurred. If err is non-nil, it will be of type
// Errors; if ExitOnError is true, err will contain the first error that
// occurred, otherwise it will contain all errors encountered during
// evaluation.
func run(f filter, opts *Options) (sums *Sums, err error) {
	var errors Errors
	f.Start()
loop:
	for {
		select {
		case <-opts.Cancel:
			f.Cancel()
			break loop
		case err, ok := <-f.Err():
			if !ok {
				break loop
			}
			if opts.ErrWriter != nil {
				_, _ = fmt.Fprintln(opts.ErrWriter, err)
			}
			errors = append(errors, err)
			if opts.ExitOnError {
				f.Cancel()
				break loop
			}
		case path, ok := <-f.Dup():
			if !ok {
				break loop
			}
			if opts.DupWriter != nil {
				_, _ = fmt.Fprintln(opts.DupWriter, path)
			}
			if opts.ExitOnDup {
				f.Cancel()
				break loop
			}
		case path, ok := <-f.Uniq():
			if !ok {
				break loop
			}
			if opts.UniqWriter != nil {
				_, _ = fmt.Fprintln(opts.UniqWriter, path)
			}
		}
	}
	sums = f.Sums()
	if len(errors) > 0 {
		err = errors
	}
	return
}

// signal provides a broadcast mechanism by exposing a receive-only channel
// that is guaranteed to be closed only once, when Once is called.
type signal struct {
	c    chan struct{}
	once *sync.Once
}

func newSignal() *signal {
	return &signal{
		c:    make(chan struct{}),
		once: new(sync.Once),
	}
}

// C returns a receive-only view of the channel managed by s. Subscribers
// will receive the zero value for the channel when s.Once is called.
func (s *signal) C() <-chan struct{} { return s.c }

// Once closes the channel managed by s the first time it is called.
// Subsequent calls of Once have no effect.
func (s *signal) Once() {
	s.once.Do(func() { close(s.c) })
}

// readLines returns an unbuffered channel on which newline-delimited text
// lines read from r are sent. The channel is closed when all lines have been
// read from r.
func readLines(r io.Reader) <-chan string {
	c := make(chan string)
	go func() {
		defer close(c)
		s := bufio.NewScanner(r)
		for s.Scan() {
			if line := s.Text(); line != "" {
				c <- line
			} else {
				break
			}
		}
	}()
	return c
}

// lstat wraps fs.Lstat, resolving symbolic links if followSymlinks is true.
// If path is a symbolic link, info will be the os.FileInfo of the linked
// file and newPath will be its path; otherwise, info will be the os.FileInfo
// of the file located at path, and newPath will be equal to path.
func lstat(fs filesys.FileSystem, path string, followSymlinks bool) (info os.FileInfo, newPath string, err error) {
	info, err = fs.Lstat(path)
	if err != nil {
		return
	}
	newPath = path
	if followSymlinks && info.Mode()&os.ModeSymlink == os.ModeSymlink {
		newPath, err = fs.Readlink(path)
		if err == nil {
			info, err = fs.Lstat(newPath)
		}
	}
	return
}

// mergeErrors returns a receive-only channel on which errors received from
// each channel in ins are sent. The channel will be closed once all values
// have been received from each channel in ins.
func mergeErrors(ins ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	out := make(chan error)
	multiplex := func(in <-chan error) {
		defer wg.Done()
		for err := range in {
			out <- err
		}
	}
	wg.Add(len(ins))
	for _, in := range ins {
		go multiplex(in)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

var maxProcs = runtime.GOMAXPROCS(0)

// ratioMaxProcs returns the greater of runtime.GOMAXPROCS(0)*n/d and 1.
func ratioMaxProcs(n, d int) int {
	if x := maxProcs * n / d; x >= 1 {
		return x
	}
	return 1
}
