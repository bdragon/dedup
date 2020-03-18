package dedup_test

import (
	"flag"
	"os"
	"testing"

	"github.com/bdragon/dedup"
)

var (
	dir = flag.String("dedup.dir", "", "The directory from which FilterDir should read.")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func BenchmarkFilterDir(b *testing.B) {
	if *dir == "" {
		b.Fatal("-dedup.dir must be specified")
	}

	opts := new(dedup.Options)
	opts.Recursive = true

	sums, err := dedup.FilterDir(*dir, opts)
	if err != nil {
		b.Fatalf("unexpected error: %v\n", err)
	}

	result := sums.Stats()

	b.Logf("Examined %d files (%d B) and found %d (%d B) duplicates.\n",
		result.NumFiles, result.NumBytes, result.NumDupFiles, result.NumDupBytes)
}

func BenchmarkFilter(b *testing.B) {
}
