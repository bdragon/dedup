package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bdragon/dedup"
)

var (
	exitOnError = flag.Bool("e", false, "If an error occurs, print it to "+
		"stderr and exit with non-zero status. The default behavior is to "+
		"print the error to stderr and continue.")

	exitOnDup = flag.Bool("b", false, "Stop processing and exit with "+
		"non-zero status if a file with a previously-seen checksum is found.")

	recursive = flag.Bool("R", false, "Read files from <dir> recursively. "+
		"Has no effect when reading from stdin.")

	followSymlinks = flag.Bool("L", false, "Follow symbolic links.")

	printUniq = flag.Bool("u", false, "Print each file with a "+
		"previously-unseen checksum to stdout.")

	printDup = flag.Bool("d", false, "Print each file with a previously-seen "+
		"checksum to stdout.")

	printAllDup = flag.Bool("D", false, "Print summary of duplicate "+
		"files and their checksums to stdout in the following format after "+
		"all files have been evaluated:\n\n"+
		"\tda39a3ee5e6b4b0d3255bfef95601890afd80709:\n"+
		"\t- \"/path/to/file1\"\n"+
		"\t- \"/path/to/file2\"\n"+
		"\t...\n")
)

func printUsageAndExit(hint string) {
	if hint != "" {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", hint)
	}

	_, _ = fmt.Fprintf(os.Stderr, "NAME\n"+
		"  dedup - detect duplicate files\n\n"+
		"SYNOPSIS\n"+
		"  dedup -u [-b] [-e] [-L] [-R] [<dir>]\n"+
		"  dedup -d [-b] [-e] [-L] [-R] [<dir>]\n"+
		"  dedup -D [-e] [-L] [-R] [<dir>]\n\n"+
		"DESCRIPTION\n"+
		"  dedup reads file paths from stdin and looks for duplicates by "+
		"computing the SHA1 checksum of each file. If <dir> is specified, "+
		"dedup evaluates files in <dir> (recursively if -R is "+
		"specified) instead.\n"+
		"  By default, nothing is printed to stdout. To print paths of files "+
		"with previously-unseen checksums to stdout, specify -u. To print "+
		"paths of files with previously-seen checksums to stdout instead, "+
		"specify -d. Or, to print a summary of all duplicate files and "+
		"their checksums to stdout once all files have been evaluated, "+
		"specify -D. Note that only one of -u, -d, and -D may be specified.\n"+
		"  After evaluating all files, dedup will exit with non-zero status "+
		"if any duplicates were found or if any errors occurred, and zero "+
		"status otherwise. By default, if an error occurs, such as failure "+
		"to open a file for reading, the error is printed to stderr and "+
		"dedup continues. This behavior may be changed by specifying -e, "+
		"which causes dedup to exit immediately if an error occurs. "+
		"Similarly, specifying -b causes dedup to exit immediately if a file "+
		"with a previously-seen checksum is encountered.\n\n"+
		"OPTIONS\n")

	flag.PrintDefaults()

	_, _ = fmt.Fprintf(os.Stderr, "\nEXAMPLES\n"+
		"  Print paths of unique images found in <dir> to stdout and "+
		"discard error messages:\n\n"+
		"    \t$ find <dir> -type f -regextype sed "+
		"-iregex '.*\\.\\(gif\\|jpe\\?g\\|png\\)' | dedup -u 2>/dev/null\n\n"+
		"  Write summary of files with duplicate checksums found in <dir> "+
		"(following any symbolic links encountered) to <file> as YAML:\n\n"+
		"    \t$ dedup -R -L -D <dir> > <file>\n\n"+
		"  Remove files with previously-seen checksums from <dir>:\n\n"+
		"    \t$ dedup -R -d <dir> | xargs rm --\n")

	os.Exit(1)
}

func main() {
	flag.Usage = func() { printUsageAndExit("") }
	flag.Parse()

	if flag.NArg() > 1 {
		printUsageAndExit("too many arguments")
	}
	if *printAllDup && *exitOnDup {
		printUsageAndExit("only one may be provided: -b, -D")
	}
	if *printUniq && *printDup || *printUniq && *printAllDup || *printDup && *printAllDup {
		printUsageAndExit("only one may be provided: -u, -d, -D")
	}

	opts := new(dedup.Options)
	opts.Recursive = *recursive
	opts.FollowSymlinks = *followSymlinks
	opts.ExitOnDup = *exitOnDup
	opts.ExitOnError = *exitOnError
	opts.ErrWriter = os.Stderr
	if *printUniq {
		opts.UniqWriter = os.Stdout
	} else if *printDup {
		opts.DupWriter = os.Stdout
	}

	cancel := make(chan struct{})
	go handleInterrupt(cancel)
	opts.Cancel = cancel

	start := time.Now()
	dir := flag.Arg(0)

	var sums *dedup.Sums
	var err error

	if dir != "" {
		sums, err = dedup.FilterDir(dir, opts)
	} else {
		sums, err = dedup.Filter(os.Stdin, opts)
	}

	if err != nil {
		os.Exit(1)
	} else {
		elapsed := time.Now().Sub(start)
		result := sums.Stats()

		_, _ = fmt.Fprintf(os.Stderr,
			"Evaluated %d files (%s) and found %d duplicates (%s) in %v.\n",
			result.NumFiles, humanSize(result.NumBytes),
			result.NumDupFiles, humanSize(result.NumDupBytes), elapsed)

		if *printAllDup {
			_ = sums.WriteAllDup(os.Stdout)
		}
		if result.NumDupFiles > 0 {
			os.Exit(1)
		}
	}

	os.Exit(0)
}

func handleInterrupt(cancel chan<- struct{}) {
	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	select {
	case <-interrupt:
		_, _ = fmt.Fprintln(os.Stderr, "Interrupted; exiting...")
		close(cancel)
	}
}

func humanSize(b uint64) string {
	unit := uint64(1000)
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div := unit
	exp := 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	const pre = "kMGTPE"
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), pre[exp])
}
