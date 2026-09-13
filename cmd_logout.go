package main

import (
	"fmt"

	"github.com/spf13/pflag"
)

func printHelpLogout() {
	fmt.Printf(
		`Usage: %[1]s logout [<opts>]

Description:
  Forget a stored API key. The key is deleted from this machine; it stays valid
  at the API, so revoke it in the console if that is what you meant.

Examples:
  $ %[1]s logout
  $ %[1]s logout --session work
  $ %[1]s logout --all

Options:
  --session <name>
    which session to forget. Default: the active one.
  --all
    forget every session.
  --help, -h
    show help.
`, progBase)
}

func cmdLogout() error {
	var fAll bool
	globalFlags()
	pflag.BoolVar(&fAll, "all", false, "forget every session.")
	parseSubFlags()

	if fHelp {
		printHelpLogout()
		return nil
	}

	if fAll {
		n := len(gConfig.Sessions)
		if n == 0 {
			fmt.Println("not logged in")
			return nil
		}
		gConfig.Sessions = map[string]*Session{}
		gConfig.Active = ""
		if err := SaveConfig(gConfig); err != nil {
			return err
		}
		fmt.Printf("forgot %d session(s)\n", n)
		return nil
	}

	name := gConfig.ActiveSessionName()
	if name == "" || gConfig.Sessions[name] == nil {
		fmt.Println("not logged in")
		return nil
	}

	delete(gConfig.Sessions, name)
	if gConfig.Active == name {
		gConfig.Active = ""
		// Falling back to whatever remains beats leaving the user
		// unauthenticated while other credentials are still stored.
		if names := gConfig.SessionNames(); len(names) > 0 {
			gConfig.Active = names[0]
		}
	}
	if err := SaveConfig(gConfig); err != nil {
		return err
	}

	fmt.Printf("forgot session %q\n", name)
	if gConfig.Active != "" {
		fmt.Printf("session %q is now active\n", gConfig.Active)
	}
	return nil
}
