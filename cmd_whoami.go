package main

import (
	"fmt"
	"os"
)

func printHelpWhoami() {
	fmt.Printf(
		`Usage: %[1]s whoami [<opts>]

Aliases: me, quota

Description:
  Which credential this machine will use, and where it came from.

  Also reports your plan, what it includes, and how much of your allowance you
  have used. That part needs the account API, which is not yet reachable from
  the client library; until it is, this reports what the CLI itself knows.

Options:
  --help, -h
    show help.
`, progBase)
}

func cmdWhoami() error {
	globalFlags()
	parseSubFlags()

	if fHelp {
		printHelpWhoami()
		return nil
	}

	key, source := gConfig.ResolveKey()
	name := gConfig.ActiveSessionName()

	if key == "" {
		fmt.Println("not authenticated")
		fmt.Printf("\nLookups still work: the free tier answers 'ip' and 'is_vpn'.\n")
		fmt.Printf("Run `%s login` to use a key, or `%s signup` to create an account.\n", progBase, progBase)
		return nil
	}

	api := gConfig.ResolveBaseURL()
	if api == "" {
		api = "https://api.vpndetection.io"
	}
	fmt.Printf("key         %s\n", maskKey(key))
	fmt.Printf("fingerprint %s\n", keyFingerprint(key))
	fmt.Printf("from        %s\n", source)
	if name != "" && gConfig.Sessions[name] != nil {
		fmt.Printf("session     %s\n", name)
	}
	fmt.Printf("api         %s\n", api)

	// Stated rather than guessed. The plan behind a key decides which fields a
	// lookup answers with, and inferring that from one lookup would report the
	// fields that ADDRESS happened to have rather than the ones the plan
	// includes - a subtly wrong answer is worse here than no answer.
	fmt.Fprintf(os.Stderr, "\nplan, entitlements and usage are not reported yet;\n")
	fmt.Fprintf(os.Stderr, "see https://app.vpndetection.io for now.\n")
	return nil
}
