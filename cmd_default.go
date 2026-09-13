package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

func printHelpDefault() {
	fmt.Printf(
		`Usage: %[1]s <ip | cidr | range | file>... [<opts>]
       %[1]s <cmd> [<opts>] [<args>]

Description:
  Look up what an address is: a VPN exit, a hosting or CDN range, a Tor node, a
  privacy relay, or a residential, datacenter or mobile proxy.

  Addresses, CIDRs, ranges and file paths are all accepted, in any mix, from
  arguments and standard input at the same time. One address prints a readable
  block; anything resolving to more than one prints JSON.

  It works with no account at all: the free tier answers 'ip' and 'is_vpn'.

Examples:
  # One address.
  $ %[1]s 45.83.91.1

  # Several, from anywhere.
  $ %[1]s 1.1.1.1 8.8.8.0/24 9.9.9.1-9.9.9.9 addresses.txt
  $ cat addresses.txt | %[1]s
  $ %[1]s 10.0.0.0/16 --csv > answers.csv

  # One field, for a script.
  $ %[1]s 45.83.91.1 -f is_vpn

Commands:
  myip        look up the address you are coming from.
  bulk        look up many addresses, always in the machine format.
  database    list, inspect and download the licensed datasets.
  signup      create an account.
  login       store an API key.
  logout      forget a stored API key.
  session     manage named credentials and switch between them.
  whoami      your key, your plan, and what you have used.
  cache       inspect or clear the local answer cache.
  config      read or change stored settings.
  completion  install shell auto-completion.
  version     print the version.

Options:
  General:
    --key <key>, -k <key>
      API key for this run, instead of the stored one.
    --session <name>
      stored session to use for this run.
    --base-url <url>
      API to talk to, instead of the default.
    --nocache
      neither read nor write the local cache.
    --concurrency <n>
      in-flight requests during a bulk lookup.
    --retries <n>
      retries per request.
    --version, -v, --vsn
      print the version.
    --help, -h
      show help.

  Output:
    --field <field>, -f <field>
      only these fields, comma separated. Dotted for nested ones, as
      'vpn.provider'. Naming an object selects all of it.
    --show-absent
      show the fields your plan does not include, rather than listing them
      at the end.
    --nocolor
      disable colored output.

  Formats:
    --pretty, -p     the readable block. (default for one address)
    --json, -j       one JSON object keyed by address. (default for many)
    --jsonl          one JSON object per line, for pipelines.
    --csv, -c        CSV with a fixed header.
    --yaml, -y       YAML. Collected in memory, so not for large runs.
`, progBase)
}

// cmdDefault handles everything that is not a subcommand: addresses to look up,
// a request for help, or a mistake.
func cmdDefault() error {
	var fVersion bool
	globalFlags()
	lookupFlags()
	pflag.BoolVarP(&fVersion, "version", "v", false, "print the version.")
	pflag.BoolVar(&fVersion, "vsn", false, "print the version.")
	resolve := formatFlags()
	args := parseFlags()

	if fVersion {
		return cmdVersion()
	}
	if fHelp {
		printHelpDefault()
		return nil
	}

	opts, err := resolve()
	if err != nil {
		return err
	}

	// No arguments and a terminal on standard input means nobody piped
	// anything and nobody named an address: that is a request for help, not an
	// invitation to sit waiting on a tty.
	if len(args) == 0 && isTerminal(os.Stdin) {
		printHelpDefault()
		return nil
	}

	err = runLookup(context.Background(), args, opts)
	if err == errNoInput {
		// The user typed SOMETHING, so say what was wrong with it rather than
		// printing the whole help text over it.
		return fmt.Errorf("no addresses found in the input; expected an IP, CIDR, range or file")
	}
	return err
}

// isTerminal reports whether a file is attached to a terminal rather than a
// pipe or a redirect.
func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}
