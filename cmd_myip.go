package main

import (
	"context"
	"fmt"
)

func printHelpMyIP() {
	fmt.Printf(
		`Usage: %[1]s myip [<opts>]

Description:
  Look up the address you are coming from.

  It is the address our edge observed, so behind a proxy or a VPN this reports
  the exit you left through rather than the machine you are typing on - which
  is usually the point of asking.

  Never served from the cache: which address you are is the whole question, and
  a laptop that moved networks would otherwise be told where it used to be.

Examples:
  $ %[1]s myip
  $ %[1]s myip -f ip
  $ %[1]s myip --json

Options:
  Takes the same output options as a lookup; run '%[1]s --help'.
`, progBase)
}

func cmdMyIP() error {
	globalFlags()
	lookupFlags()
	resolve := formatFlags()
	args := parseSubFlags()

	if fHelp {
		printHelpMyIP()
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("myip takes no arguments; did you mean `%s %s`?", progBase, args[0])
	}

	opts, err := resolve()
	if err != nil {
		return err
	}

	client, err := NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	result, err := client.api.MyIP(context.Background())
	if err != nil {
		return explain(err)
	}
	return writeOne(result.IP, result, opts)
}
