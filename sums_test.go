package dedup

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

var (
	keys = []string{
		"aqua", "black", "blue", "fuchsia", "gray", "green", "lime", "maroon",
		"navy", "olive", "purple", "red", "silver", "teal", "white", "yellow",
	}
	sumKey = make(map[Sum]string, len(keys))
	keySum = make(map[string]Sum, len(keys))
)

func init() {
	for _, key := range keys {
		sum := sha1.Sum([]byte(key))
		sumKey[sum] = key
		keySum[key] = sum
	}
}

func TestSumsConcurrent(t *testing.T) {
	const P = 8

	sums := NewSums()

	var wg sync.WaitGroup
	wg.Add(1 + P)

	go func() { // Add 1 file for keys[0]
		defer wg.Done()
		key := keys[0]
		sums.Append(keySum[key],
			fakeFile(fmt.Sprintf("/dir0/%s", key), key))
	}()

	for i := 0; i < P; i++ { // Add P files for each of keys[1:]
		go func(i int) {
			defer wg.Done()
			for _, key := range keys[1:] {
				sums.Append(keySum[key],
					fakeFile(fmt.Sprintf("/dir%d/%s", i+1, key), key))
			}
		}(i)
	}

	wg.Wait()

	seen := make(map[string][]string) // keys to file paths
	sums.Range(func(sum Sum, files []*File) bool {
		key, ok := sumKey[sum]
		if !ok {
			t.Errorf("unwanted checksum: %x", sum)
		}
		seen[key] = make([]string, len(files))
		for i := range files {
			seen[key][i] = files[i].Path
		}
		return true
	})

	for i, key := range keys {
		if files, ok := seen[key]; !ok {
			t.Errorf("want Sums to have checksum %x, but it did not",
				keySum[key])
		} else if i == 0 && len(files) != 1 {
			t.Errorf("want Sums to have 1 file for checksum %x; got %d files",
				keySum[key], len(files))
		} else if i > 0 && len(files) != P {
			t.Errorf("want Sums to have %d files for checksum %x; got %d files",
				P, keySum[key], len(files))
		}
	}

	want := Stats{}
	want.NumFiles = 1                    // 1 file added for keys[0]
	want.NumBytes = uint64(len(keys[0])) // len(keys[0]) bytes added for keys[0]
	for _, key := range keys[1:] {
		want.NumFiles += P
		want.NumBytes += P * uint64(len(key))
		want.NumDupFiles += P - 1
		want.NumDupBytes += (P - 1) * uint64(len(key))
	}
	if got := sums.Stats(); !reflect.DeepEqual(want, got) {
		t.Errorf("Stats() = %v; want %v", got, want)
	}
}

func TestSumsAppend(t *testing.T) {
	sums := NewSums()
	sum1, sum2 := keySum[keys[0]], keySum[keys[1]]
	emptyFile := fakeFile("", "")

	if dup := sums.Append(sum1, emptyFile); dup {
		t.Errorf("Append(%x, ...) = true; want false", sum1)
	}
	if dup := sums.Append(sum1, emptyFile); !dup {
		t.Errorf("Append(%x, ...) = false; want true", sum1)
	}

	// Test concurrently:

	const P = 8
	done := make(chan bool)

	var mu sync.Mutex // Protect access to added
	var added bool

	for i := 0; i < P; i++ {
		go func() {
			mu.Lock()
			dup := sums.Append(sum2, emptyFile)
			added1 := added
			added = true
			mu.Unlock()

			if !added1 && dup {
				t.Errorf("Append(%x, ...) = true; want false", sum2)
			} else if added1 && !dup {
				t.Errorf("Append(%x, ...) = false; want true", sum2)
			}
			done <- true
		}()
	}

	for i := 0; i < P; i++ {
		<-done
	}
	close(done)
}

func TestSumsWriteAllDup(t *testing.T) {
	uniqKeys, dupKeys := keys[:8], keys[8:]
	sums := NewSums()
	want := make([]string, 8*3)

	for _, key := range uniqKeys { // Add 1 file for each of uniqKeys
		sums.Append(keySum[key], fakeFile(fmt.Sprintf("/%s/file1", key), ""))
	}

	paths := make([]string, 3)
	for i, key := range dupKeys { // Add 3 files for each of dupKeys
		for j := 0; j < 3; j++ {
			paths[j] = fmt.Sprintf("/%s/file%d", key, j+1)
			sums.Append(keySum[key], fakeFile(paths[j], ""))
		}
		want[8+i] = dupString(keySum[key], paths...)
	}

	checkSums(t, "", sums, want)
}

// info implements os.FileInfo for testing.
type info struct {
	name string
	size int
}

var _ os.FileInfo = (*info)(nil)

func (i *info) Name() string       { return i.name }
func (i *info) Size() int64        { return int64(i.size) }
func (i *info) Mode() os.FileMode  { return 0 }
func (i *info) ModTime() time.Time { return time.Time{} }
func (i *info) IsDir() bool        { return false }
func (i *info) Sys() interface{}   { return nil }

func fakeFile(path string, contents string) *File {
	return &File{
		Path: path,
		Info: &info{
			name: filepath.Base(path),
			size: len(contents),
		},
	}
}
