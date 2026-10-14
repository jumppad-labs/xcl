// Command xcl-release makes the decisions behind automated releases for xcl
// and every language binding. The release workflows run it; the rules live
// in internal/release, where they are tested.
//
//	xcl-release labels <label>...            print the release size the labels name
//	xcl-release next --size <size> <tag>...  print the tag a release of that size makes
//	xcl-release bump <from-tag> <to-tag>     print the size of the release between two tags
//
// labels fails when the labels name no release size or more than one. next
// prints nothing for no-release, and fails when no tag is a version. Every
// result is printed on standard output; errors go to standard error.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jumppad-labs/xcl/internal/release"
)

// Exit statuses of the program.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

const usage = `usage:
  xcl-release labels <label>...
  xcl-release next --size <major|minor|patch|no-release> <tag>...
  xcl-release bump <from-tag> <to-tag>`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run runs the subcommand named by the first argument and returns the exit
// status.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return exitUsage
	}

	switch args[0] {
	case "labels":
		return runLabels(args[1:], stdout, stderr)
	case "next":
		return runNext(args[1:], stdout, stderr)
	case "bump":
		return runBump(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, usage)
		return exitUsage
	}
}

// runLabels prints the release size the labels name.
func runLabels(labels []string, stdout, stderr io.Writer) int {
	size, err := release.SizeFromLabels(labels)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}

	fmt.Fprintln(stdout, size)
	return exitOK
}

// runNext prints the tag a release of the given size makes after the latest
// version among the tags, or nothing for no-release.
func runNext(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("next", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sizeFlag := flags.String("size", "", "the release size: major, minor, patch or no-release")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	size, err := release.SizeFromLabels([]string{*sizeFlag})
	if err != nil {
		fmt.Fprintf(stderr, "--size %q is not a release size\n", *sizeFlag)
		return exitUsage
	}

	next, ok, err := release.Next(flags.Args(), size)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	if ok {
		fmt.Fprintln(stdout, next)
	}
	return exitOK
}

// runBump prints the size of the release between two version tags.
func runBump(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, usage)
		return exitUsage
	}

	size, err := release.Bump(args[0], args[1])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}

	fmt.Fprintln(stdout, size)
	return exitOK
}
