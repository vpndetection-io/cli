package main

import (
	"errors"
	"fmt"
	"strings"
)

func printHelpCache() {
	fmt.Printf(
		`Usage: %[1]s cache <info | clear>

Description:
  The local answer cache, which is what makes looking the same address up twice
  cost one request instead of two.

  Entries are partitioned by CREDENTIAL, not just by address: which fields an
  answer carries depends on the plan behind the key, so one key's answers are
  never served to another's. Clearing empties every partition.

  Turn it off for one run with --nocache, or for good with
  '%[1]s config cache=disable'.

Subcommands:
  info
    where the cache is, how big it is, and how much of it has expired.
  clear
    empty it, across every credential.

Examples:
  $ %[1]s cache info
  $ %[1]s cache clear

Options:
  --help, -h
    show help.
`, progBase)
}

func cmdCache() error {
	globalFlags()
	args := parseSubFlags()

	// A bare `cache` prints help rather than the report, matching every other
	// subcommand here; `cache info` is the report.
	if fHelp || len(args) == 0 {
		printHelpCache()
		return nil
	}
	if len(args) != 1 {
		return errors.New("usage: cache <info | clear>")
	}

	switch strings.ToLower(args[0]) {
	case "info":
		stats, err := InspectCache(gConfig.cacheTTL())
		if err != nil {
			return err
		}
		state := "enabled"
		if !gConfig.CacheEnabled {
			state = "disabled"
		}
		fmt.Printf("state       %s\n", state)
		fmt.Printf("ttl         %s\n", gConfig.cacheTTL())
		fmt.Printf("path        %s\n", stats.Path)
		fmt.Printf("size        %s\n", humanBytes(stats.Size))
		fmt.Printf("credentials %d\n", stats.Buckets)
		fmt.Printf("entries     %d (%d expired)\n", stats.Entries, stats.Expired)
		return nil
	case "clear":
		if err := ClearCache(); err != nil {
			return err
		}
		fmt.Println("cache cleared")
		return nil
	default:
		printHelpCache()
		return fmt.Errorf("%q is not a cache subcommand", args[0])
	}
}

// humanBytes renders a byte count for a person.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
