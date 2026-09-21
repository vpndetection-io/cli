// Command vpndetection is the official command line for the VPNDetection API:
// look up an address, look up a great many, and download the licensed datasets.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

// version is the release, set here and checked against the tag at release time.
var version = "1.3.0"

// progBase is the binary as the user invoked it, so every help string and
// example names what they actually typed. Installing the binary under a
// different name therefore produces correct help for free.
var progBase = filepath.Base(os.Args[0])

// Global flags, registered by globalFlags and shared by every command.
var (
	fHelp        bool
	fNoColor     bool
	fNoCache     bool
	fKey         string
	fSession     string
	fBaseURL     string
	fConcurrency int
	fRetries     = -1
)

func main() {
	// NO_COLOR is the cross-tool convention and costs nothing to honour.
	if os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}

	// Answered before anything else: the shell invokes the binary with
	// COMP_LINE set and reads candidates from stdout, so nothing else may
	// print first.
	handleCompletions()

	var err error
	switch detectCommand() {
	case "myip":
		err = cmdMyIP()
	case "bulk":
		err = cmdBulk()
	case "signup":
		err = cmdSignup()
	case "login", "init":
		err = cmdLogin()
	case "logout":
		err = cmdLogout()
	case "session", "sessions":
		err = cmdSession()
	case "whoami", "entitlement":
		err = cmdWhoami()
	case "database", "db":
		err = cmdDatabase()
	case "cache":
		err = cmdCache()
	case "config":
		err = cmdConfig()
	case "completion":
		err = cmdCompletion()
	case "version", "vsn", "v":
		err = cmdVersion()
	default:
		// Not a subcommand, so it is either addresses to look up, a request for
		// help, or a mistake. cmdDefault tells those apart.
		err = cmdDefault()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "err: %v\n", err)
		os.Exit(1)
	}
}

// subcommands is every command name, including aliases.
var subcommands = map[string]bool{
	"myip": true, "bulk": true, "signup": true, "login": true, "init": true, "logout": true,
	"session": true, "sessions": true, "whoami": true, "entitlement": true,
	"database": true, "db": true, "cache": true, "config": true,
	"completion": true, "version": true, "vsn": true, "v": true,
}

// valueFlags are the global flags that consume the argument after them, so a
// session named "database" is not mistaken for the database command.
var valueFlags = map[string]bool{
	"--key": true, "-k": true, "--session": true, "--base-url": true,
	"--format": true, "--field": true, "-f": true,
	"--concurrency": true, "--retries": true, "--limit": true,
}

// commandArg is the index in os.Args of the subcommand, or 0 for none.
//
// Scanned rather than read from os.Args[1], because a flag may come first:
// `vpndetection --session work db list` is a natural thing to type, and taking
// the second argument literally makes it a lookup of the address "--session".
// The value-taking flags are skipped so that `--session database` names a
// session rather than invoking a command.
func commandArg() int {
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "-") {
			// --flag=value carries its value, so nothing after it is consumed.
			if valueFlags[arg] && !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		if subcommands[arg] {
			return i
		}
		// The first positional that is not a command is an address, and a
		// command cannot follow it.
		return 0
	}
	return 0
}

// detectCommand is the subcommand being run, or "" when this is a lookup.
func detectCommand() string {
	if i := commandArg(); i > 0 {
		return os.Args[i]
	}
	return ""
}

// globalFlags registers the options every command accepts.
//
// pflag's command line is global and a command registers its own flags on top,
// which is why this is a function each command calls rather than an init: two
// commands registering the same flag name would panic at startup.
func globalFlags() {
	pflag.BoolVarP(&fHelp, "help", "h", false, "show help.")
	pflag.BoolVar(&fNoColor, "nocolor", false, "disable colored output.")
	pflag.StringVarP(&fKey, "key", "k", "", "API key to use for this run.")
	pflag.StringVar(&fSession, "session", "", "named session to use for this run.")
	pflag.StringVar(&fBaseURL, "base-url", "", "API base URL to use for this run.")
}

// lookupFlags adds the options every command that reaches the lookup API takes.
func lookupFlags() {
	pflag.BoolVar(&fNoCache, "nocache", false, "do not read or write the cache.")
	pflag.IntVar(&fConcurrency, "concurrency", 0, "in-flight requests during a bulk lookup.")
	pflag.IntVar(&fRetries, "retries", -1, "retries per request.")
}

// parseFlags parses, applies whatever the global flags imply, and returns the
// positional arguments.
//
// Nothing is stripped: the default command's positionals are the addresses to
// look up, and dropping the first one silently loses an address whenever the
// user names more than one.
func parseFlags() []string {
	pflag.Parse()
	if fNoColor {
		color.NoColor = true
	}
	return pflag.Args()
}

// parseSubFlags is parseFlags for a named subcommand, minus the subcommand
// itself, which pflag hands back as the first positional.
func parseSubFlags() []string {
	args := parseFlags()
	if i := commandArg(); i > 0 && len(args) > 0 && args[0] == os.Args[i] {
		return args[1:]
	}
	return args
}
