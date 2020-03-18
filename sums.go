package dedup

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
)

// Sum is a type alias for [sha1.Size]byte.
type Sum [sha1.Size]byte

// File pairs a path with the os.FileInfo for the file located at that path.
type File struct {
	Path string
	Info os.FileInfo
}

// Stats contains a summary of files and bytes examined by Sums.
type Stats struct {
	NumFiles    uint64
	NumBytes    uint64
	NumDupFiles uint64
	NumDupBytes uint64
}

func (s Stats) String() string {
	return fmt.Sprintf("%d (%d B) duplicate files / %d (%d B) total files",
		s.NumDupFiles, s.NumDupBytes, s.NumFiles, s.NumBytes)
}

// Sums is a map of checksums to files that is safe for concurrent access from
// multiple goroutines.
type Sums struct {
	mu sync.Mutex
	m  map[Sum][]*File
	r  Stats
}

// NewSums initializes a Sums and returns a pointer to it.
func NewSums() *Sums {
	s := new(Sums)
	s.m = make(map[Sum][]*File)
	return s
}

// Get returns the list of files for sum. ok will be false if s does not
// contain any files for sum, true otherwise.
func (s *Sums) Get(sum Sum) (files []*File, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, ok = s.m[sum]
	return
}

// Append stores file in the set of files under checksum sum. Append does not
// attempt to verify whether sum is a valid checksum for file. Append returns
// false if file is the first encountered for sum, true otherwise.
func (s *Sums) Append(sum Sum, file *File) (dup bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	numBytes := uint64(file.Info.Size())

	s.r.NumFiles++
	s.r.NumBytes += numBytes

	if files, ok := s.m[sum]; ok {
		s.m[sum] = append(files, file)
		s.r.NumDupFiles++
		s.r.NumDupBytes += numBytes
		dup = true
	} else {
		s.m[sum] = []*File{file}
	}
	return
}

// Range calls f sequentially for each sum and set of files present in s. If
// f returns false, Range stops the iteration. If s is modified concurrently,
// Range may reflect any mapping for a given key during the Range call.
func (s *Sums) Range(f func(sum Sum, files []*File) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for sum, files := range s.m {
		if !f(sum, files) {
			break
		}
	}
}

// Stats reports the number of files, bytes, duplicate files, and duplicate
// bytes examined.
func (s *Sums) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.r
}

// WriteAllDup writes a summary of duplicate files and their checksums to w
// in the following format:
//
//	da39a3ee5e6b4b0d3255bfef95601890afd80709:
//	- "/path/to/file1"
//	- "/path/to/file2"
//	...
func (s *Sums) WriteAllDup(w io.Writer) (err error) {
	s.Range(func(sum Sum, files []*File) bool {
		if len(files) > 1 {
			_, err = fmt.Fprintf(w, "%x:\n", sum)
			if err != nil {
				return false
			}
			paths := sortedPaths(files)
			for _, path := range paths {
				_, err = fmt.Fprintf(w, "- %q\n", path)
				if err != nil {
					return false
				}
			}
		}
		return true
	})
	return
}

func sortedPaths(files []*File) []string {
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	sort.Strings(paths)
	return paths
}
