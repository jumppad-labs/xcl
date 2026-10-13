// Command xcl-core is the xcl engine as a program a host in any language
// starts and talks to, the transport every language binding ships. With
// --serve it serves CoreService over its standard input and output: the host
// writes Request frames, a 4-byte big-endian length followed by the message,
// and reads Response frames back, as proto/xcl/core/v1/transport.proto
// defines them. The program moves bytes between frames and
// coreservice.Service and holds no logic of its own; see the coreservice
// package for the behaviour of every message. It is built with cgo disabled,
// so it cross-compiles for every platform from one machine.
//
// Standard output carries frames only. Before serving, the program keeps its
// standard input and output for the protocol, points os.Stdin at the null
// device, so an external plugin it starts cannot read frames, and points
// os.Stdout at standard error, where all of its own diagnostics go.
//
// The host decides what an interrupt cancels: the program discards SIGINT,
// and on Windows Ctrl-C and Ctrl-Break, rather than ignoring them, since an
// ignored console control event on Windows ends the process. End of input or
// SIGTERM releases every configuration, which saves the state of any
// operation in progress, and exits 0. A framing error exits 1.
//
//	xcl-core --serve      serve CoreService over stdin and stdout until stdin ends or SIGTERM
//	xcl-core --version    print the stamped version and exit 0
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jumppad-labs/xcl/coreservice"
)

// Exit statuses of the program.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses the command line and runs the program, returning its exit
// status. stdout receives the version, stderr the usage and every error.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("xcl-core", flag.ContinueOnError)
	flags.SetOutput(stderr)
	serveFlag := flags.Bool("serve", false, "serve CoreService over standard input and output until input ends")
	versionFlag := flags.Bool("version", false, "print the engine's version and exit")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: xcl-core --serve | --version")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	switch {
	case *versionFlag:
		fmt.Fprintln(stdout, coreservice.Version)
		return exitOK
	case *serveFlag:
		return serveStdio(stderr)
	default:
		flags.Usage()
		return exitUsage
	}
}

// serveStdio takes over the process's standard input and output for the
// protocol and serves until input ends or the program is asked to terminate.
func serveStdio(stderr io.Writer) int {
	in, out := os.Stdin, os.Stdout

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		fmt.Fprintf(stderr, "xcl-core: unable to open %s: %v\n", os.DevNull, err)
		return exitError
	}
	defer devNull.Close()

	// go-plugin hands os.Stdin to every external plugin it starts, and
	// nothing but frames may reach the protocol's output
	os.Stdin = devNull
	os.Stdout = os.Stderr

	discardSignals()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

	if err := serve(ctx, in, out, coreservice.New()); err != nil {
		fmt.Fprintf(stderr, "xcl-core: %v\n", err)
		return exitError
	}

	return exitOK
}

// discardSignals catches and drops interrupts, so the host decides what to
// cancel, and SIGPIPE, so a host that has gone away turns a write into an
// error rather than ending the program before its configurations are
// released. On Windows Go reports both CTRL_C_EVENT and CTRL_BREAK_EVENT as
// os.Interrupt; signal.Ignore is not used because an ignored console control
// event falls through to the default handler, which ends the process.
func discardSignals() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGPIPE)

	go func() {
		for range signals {
		}
	}()
}
