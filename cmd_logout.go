package main

import (
	"context"
	"fmt"
	"time"

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
		for _, s := range gConfig.Sessions {
			revokeSession(s)
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

	revokeSession(gConfig.Sessions[name])
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

// revokeSession tells the server to forget a browser-issued credential, so
// logging out ends the grant rather than only deleting this machine's copy of
// it. A session created by pasting a key has no refresh token and nothing to
// revoke - the key itself stays valid, which is correct, because the user
// created it elsewhere and we were only holding it.
//
// FAILURES ARE IGNORED ON PURPOSE. `logout` must always succeed locally: a
// machine that is offline, or whose token has already expired, still needs its
// stored credential gone. Refusing to log out because the network is down is
// the wrong answer to "remove this from my laptop".
func revokeSession(s *Session) {
	if s == nil || s.RefreshToken == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = revokeToken(ctx, s.RefreshToken)
}
