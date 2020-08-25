# uniq

[![Build Status](https://travis-ci.org/bdragon/uniq.svg?branch=master)](https://travis-ci.org/bdragon/uniq)
[![Go Report Card](https://goreportcard.com/badge/github.com/bdragon/uniq)](https://goreportcard.com/report/github.com/bdragon/uniq)
[![Documentation](https://godoc.org/github.com/bdragon/uniq?status.svg)](http://godoc.org/github.com/bdragon/uniq)
[![Latest release](https://img.shields.io/github/release/bdragon/uniq/all)](https://github.com/bdragon/uniq/releases)
[![BSD-3-Clause License](https://img.shields.io/github/license/bdragon/uniq.svg)](LICENSE)

`uniq` is a command-line utility for detecting duplicate files.

## Usage

```
NAME
  uniq - detect duplicate files

SYNOPSIS
  uniq -u [-b] [-e] [-L] [-R] [<dir>]
  uniq -d [-b] [-e] [-L] [-R] [<dir>]
  uniq -D [-e] [-L] [-R] [<dir>]

DESCRIPTION
  uniq reads file paths from stdin and looks for duplicates by computing the 
SHA1 checksum of each file. If <dir> is specified, uniq evaluates files in 
<dir> (recursively if -R is specified) instead.
  By default, nothing is printed to stdout. To print paths of files with 
previously-unseen checksums to stdout, specify -u. To print paths of files 
with previously-seen checksums to stdout instead, specify -d. Or, to print a 
summary of all duplicate files and their checksums to stdout once all files 
have been evaluated, specify -D. Note that only one of -u, -d, and -D may 
be specified.
  After evaluating all files, uniq will exit with non-zero status if any 
duplicates were found or if any errors occurred, and zero status otherwise. 
By default, if an error occurs, such as failure to open a file for reading, 
the error is printed to stderr and uniq continues. This behavior may be 
changed by specifying -e, which causes uniq to exit immediately if an error 
occurs. Similarly, specifying -b causes uniq to exit immediately if a file 
with a previously-seen checksum is encountered.

OPTIONS
  -D	Print summary of duplicate files and their checksums to stdout in 
    	the following format after all files have been evaluated:

    		da39a3ee5e6b4b0d3255bfef95601890afd80709:
    		- "/path/to/file1"
    		- "/path/to/file2"
    		...

  -L	Follow symbolic links.
  -R	Read files from <dir> recursively. Has no effect when reading from 
    	stdin.
  -b	Stop processing and exit with non-zero status if a file with a 
    	previously-seen checksum is found.
  -d	Print each file with a previously-seen checksum to stdout.
  -e	If an error occurs, print it to stderr and exit with non-zero status. 
    	The default behavior is to print the error to stderr and continue.
  -u	Print each file with a previously-unseen checksum to stdout.

EXAMPLES
  Print paths of unique images found in <dir> to stdout and discard error 
messages:

    	$ find <dir> -type f -regextype sed \
    		-iregex '.*\.\(gif\|jpe\?g\|png\)' | uniq -u 2>/dev/null

  Write summary of files with duplicate checksums found in <dir> (following 
any symbolic links encountered) to <file> as YAML:

    	$ uniq -R -L -D <dir> > <file>

  Remove files with previously-seen checksums from <dir>:

    	$ uniq -R -d <dir> | xargs rm --
```
