package dedup

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/bdragon/dedup/filesys"
)

var (
	Dup1    = randBytes(1e6)
	Dup2    = randBytes(1e6)
	Dup3    = randBytes(1e6)
	Dup1Sum = sha1.Sum(Dup1)
	Dup2Sum = sha1.Sum(Dup2)
	Dup3Sum = sha1.Sum(Dup3)

	Files = map[string][]byte{
		"dup1":                 Dup1,
		"other/dup3":           Dup3,
		"other/lime":           []byte("lime"),
		"root/black":           []byte("black"),
		"root/dup2":            Dup2,
		"root/err":             nil,
		"root/foo/bar/dup1":    Dup1,
		"root/foo/baz/err":     nil,
		"root/foo/bar/green":   []byte("green"),
		"root/foo/baz/dup2":    Dup2,
		"root/foo/baz/yellow":  []byte("yellow"),
		"root/foo/blue":        []byte("blue"),
		"root/foo/dup3":        Dup3,
		"root/foo/err":         nil,
		"root/link":            []byte("dup1"), // symlink => dup1
		"root/red":             []byte("red"),
		"root/qux/quux/aqua":   []byte("aqua"),
		"root/qux/quux/dup1":   Dup1,
		"root/qux/quux/link":   []byte("other"), // symlink => other
		"root/qux/quuz/dup2":   Dup2,
		"root/qux/quuz/err":    nil,
		"root/qux/quuz/purple": []byte("purple"),
		"root/qux/dup3":        Dup3,
		"root/qux/err":         nil,
		"root/qux/fuchsia":     []byte("fuchsia"),
	}

	FS filesys.FileSystem = testFS{
		filesys.Map(Files, []string{"root/link", "root/qux/quux/link"}),
		map[string]string{
			"root/foo/baz/err":  "open root/foo/baz/err: permission denied",
			"root/foo/err":      "open root/foo/err: permission denied",
			"root/qux/quuz/err": "open root/qux/quuz/err: permission denied",
			"root/qux/err":      "open root/qux/err: permission denied",
			"root/err":          "open root/err: permission denied",
		},
	}
)

type testFS struct {
	filesys.FileSystem
	errs map[string]string
}

func (fs testFS) Open(path string) (filesys.File, error) {
	if s, ok := fs.errs[path]; ok {
		return nil, errors.New(s)
	}
	return fs.FileSystem.Open(path)
}

// dupString returns a string for sum and paths in the format
// used by WriteAllDup.
func dupString(sum Sum, paths ...string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%x:\n", sum))
	for _, path := range paths {
		b.WriteString(fmt.Sprintf("- %q\n", path))
	}
	return b.String()
}

func randBytes(n int64) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func pathReader(paths ...string) io.Reader {
	return strings.NewReader(strings.Join(paths, "\n") + "\n")
}

func TestFilter(t *testing.T) {
	tests := []struct {
		r     io.Reader
		opts  *Options
		check func(*Sums, error)
	}{
		{
			r:    strings.NewReader(""),
			opts: &Options{fs: FS},
			check: func(sums *Sums, err error) {
				if got := sums.Stats().NumFiles; got != 0 {
					t.Errorf("1: Stats().NumFiles = %d; want 0", got)
				}
				if err != nil {
					t.Errorf("1: unexpected error: %v", err)
				}
			},
		},
		{
			r: pathReader(
				"root/black",
				"root/dup2",
				"root/err",
				"root/foo/bar/dup1",
				"root/foo/baz/err",
				"root/foo/bar/green",
				"root/foo/baz/dup2",
				"root/foo/baz/yellow",
				"root/foo/blue",
				"root/foo/dup3",
				"root/foo/err",
				"root/link",
				"root/red",
				"root/qux/quux/aqua",
				"root/qux/quux/dup1",
				"root/qux/quux/link",
				"root/qux/quuz/dup2",
				"root/qux/quuz/err",
				"root/qux/quuz/purple",
				"root/qux/dup3",
				"root/qux/err",
				"root/qux/fuchsia",
			),
			opts: &Options{FollowSymlinks: true, fs: FS},
			check: func(sums *Sums, err error) {
				want := uint64(16) // root/**/* = 22 files, less 5 errors, less 1 symlink to a directory
				if got := sums.Stats().NumFiles; got != want {
					t.Errorf("2: Stats().NumFiles = %d; want %d", got, want)
				}
				checkSums(t, "2: ", sums, []string{
					dupString(Dup1Sum, "dup1", "root/foo/bar/dup1", "root/qux/quux/dup1"),
					dupString(Dup2Sum, "root/dup2", "root/foo/baz/dup2", "root/qux/quuz/dup2"),
					dupString(Dup3Sum, "root/foo/dup3", "root/qux/dup3"),
				})
				checkErrors(t, "2: ", err, []string{
					"open root/foo/baz/err: permission denied",
					"open root/foo/err: permission denied",
					"open root/qux/quuz/err: permission denied",
					"open root/qux/err: permission denied",
					"open root/err: permission denied",
				})
			},
		},
	}
	for _, tt := range tests {
		tt.check(Filter(tt.r, tt.opts))
	}
}

func TestFilterDir(t *testing.T) {
	tests := []struct {
		path  string
		opts  *Options
		check func(*Sums, error)
	}{
		{
			path: "bogus",
			opts: &Options{fs: FS},
			check: func(sums *Sums, err error) {
				if err == nil || err.Error() != "file does not exist" {
					t.Errorf("1: got %v; want file does not exist", err)
				}
			},
		},
		{
			path: "root",
			opts: &Options{fs: FS},
			check: func(sums *Sums, err error) {
				want := uint64(4) // root/{black,dup2,link,red}
				if got := sums.Stats().NumFiles; got != want {
					t.Errorf("2: Stats().NumFiles = %d; want %d", got, want)
				}
				checkErrors(t, "2: ", err, []string{
					"open root/err: permission denied",
				})
			},
		},
		{
			path: "root",
			opts: &Options{FollowSymlinks: true, fs: FS},
			check: func(sums *Sums, err error) {
				want := uint64(4) // dup1, root/{black,dup2,red}
				if got := sums.Stats().NumFiles; got != want {
					t.Errorf("3: Stats().NumFiles = %d; want %d", got, want)
				}
				checkErrors(t, "3: ", err, []string{
					"open root/err: permission denied",
				})
			},
		},
		{
			path: "root",
			opts: &Options{Recursive: true, fs: FS},
			check: func(sums *Sums, err error) {
				checkSums(t, "4: ", sums, []string{
					dupString(Dup1Sum, "root/foo/bar/dup1", "root/qux/quux/dup1"),
					dupString(Dup2Sum, "root/dup2", "root/foo/baz/dup2", "root/qux/quuz/dup2"),
					dupString(Dup3Sum, "root/foo/dup3", "root/qux/dup3"),
				})
				checkErrors(t, "4: ", err, []string{
					"open root/foo/baz/err: permission denied",
					"open root/foo/err: permission denied",
					"open root/qux/quuz/err: permission denied",
					"open root/qux/err: permission denied",
					"open root/err: permission denied",
				})
			},
		},
		{
			path: "root",
			opts: &Options{Recursive: true, FollowSymlinks: true, fs: FS},
			check: func(sums *Sums, err error) {
				checkSums(t, "5: ", sums, []string{
					dupString(Dup1Sum, "dup1", "root/foo/bar/dup1", "root/qux/quux/dup1"),
					dupString(Dup2Sum, "root/dup2", "root/foo/baz/dup2", "root/qux/quuz/dup2"),
					dupString(Dup3Sum, "other/dup3", "root/foo/dup3", "root/qux/dup3"),
				})
				checkErrors(t, "5: ", err, []string{
					"open root/foo/baz/err: permission denied",
					"open root/foo/err: permission denied",
					"open root/qux/quuz/err: permission denied",
					"open root/qux/err: permission denied",
					"open root/err: permission denied",
				})
			},
		},
	}
	for _, tt := range tests {
		tt.check(FilterDir(tt.path, tt.opts))
	}
}

func checkSums(t *testing.T, prefix string, sums *Sums, want []string) {
	var buf bytes.Buffer
	if err := sums.WriteAllDup(&buf); err != nil {
		t.Errorf("%sWriteAllDup() = %v", prefix, err)
		return
	}

	s := buf.String()
	for _, dup := range want {
		if i := strings.Index(s, dup); i >= 0 {
			s = s[:i] + s[i+len(dup):] // Found dup; remove it from s
		} else {
			t.Errorf("%swant WriteAllDup to write:\n%s", prefix, dup)
		}
	}
	if s != "" {
		t.Errorf("%sdid not want WriteAllDup to write:\n%s", prefix, s)
	}
}

func checkErrors(t *testing.T, prefix string, err error, want []string) {
	if want == nil {
		if err != nil {
			t.Errorf("%serr = %#v; want <nil>", prefix, err)
		}
		return
	}

	errs, ok := err.(Errors)
	if !ok {
		t.Errorf("%swant err.(Errors); got %#v", prefix, err)
		return
	}

	seen := make(map[string]bool)
	for _, got := range errs {
		g := got.Error()
		for _, w := range want {
			if g == w {
				seen[g] = true
				break
			}
		}
		if !seen[g] {
			t.Errorf("%sdid not want err.(Errors) to include: %#v", prefix, g)
		}
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("%swant err.(Errors) to include: %s", prefix, w)
		}
	}
}
