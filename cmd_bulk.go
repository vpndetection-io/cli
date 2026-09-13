package main

import (
	"context"
	"fmt"

	iputil "github.com/mslmio/libgo-iputil"
)

func printHelpBulk() {
	fmt.Printf(
		`Usage: %[1]s bulk <ip | cidr | range | file>... [<opts>]

Description:
  Look up many addresses. Identical to the default command except that the
  output shape does not change with the answer count, which is what a script
  wants: one address still prints the machine format rather than the readable
  block.

  With no arguments and a terminal, it reads addresses typed one per line and
  stops at a blank line.

Examples:
  $ %[1]s bulk 8.8.8.0/24
  $ %[1]s bulk 1.1.1.1 8.8.8.8 --csv
  $ cat addresses.txt | %[1]s bulk --jsonl

Options:
  Takes every option the default command takes; run '%[1]s --help'.
`, progBase)
}

func cmdBulk() error {
	globalFlags()
	lookupFlags()
	resolve := formatFlags()
	args := parseSubFlags()

	if fHelp {
		printHelpBulk()
		return nil
	}

	opts, err := resolve()
	if err != nil {
		return err
	}
	opts.forceBulk = true
	// Prompting is right here and nowhere else: this command exists to consume
	// a list, so a bare `bulk` at a terminal means "let me type one".
	opts.input = iputil.Opts{Stdin: true, Files: true, Interactive: true}

	err = runLookup(context.Background(), args, opts)
	if err == errNoInput {
		return fmt.Errorf("no addresses found in the input; expected an IP, CIDR, range or file")
	}
	return err
}
