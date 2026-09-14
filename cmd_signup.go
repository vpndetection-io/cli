package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/pflag"
)

// signupURL is where an account is created.
//
// Derived from the session's API host rather than fixed, so a session pointed
// at another deployment sends you to that deployment's console instead of to
// production - the one way this could quietly cost someone a real account they
// did not want. Anything we do not recognise falls back to production.
func signupURL() string {
	const prod = "https://app.vpndetection.io/auth/signup"
	base := gConfig.ResolveBaseURL()
	if base == "" {
		return prod
	}
	u, err := url.Parse(base)
	if err != nil || !strings.HasSuffix(u.Hostname(), ".vpndetection.io") {
		return prod
	}
	rest, ok := strings.CutPrefix(u.Hostname(), "api")
	if !ok {
		return prod
	}
	return "https://app" + rest + "/auth/signup"
}

func printHelpSignup() {
	fmt.Printf(
		`Usage: %[1]s signup [<opts>]

Description:
  Open the console to create an account, then store the API key it gives you.

  Sign up in the browser, create a key on the API page, and paste it back here;
  the prompt does not echo and does not reach your shell history. If no browser
  opens, the URL is printed.

  You do not need an account to use this tool: the free tier answers 'ip' and
  'is_vpn' with no key at all.

Options:
  --no-browser
    print the URL instead of opening it.
  --help, -h
    show help.
`, progBase)
}

func cmdSignup() error {
	var fNoBrowser bool
	globalFlags()
	pflag.BoolVar(&fNoBrowser, "no-browser", false, "print the URL instead of opening it.")
	parseSubFlags()

	if fHelp {
		printHelpSignup()
		return nil
	}

	url := signupURL()
	opened := false
	if !fNoBrowser {
		// Errors are ignored on purpose: a headless machine has no browser, and
		// the printed URL below is the answer in both cases.
		opened = browser.OpenURL(url) == nil
	}
	if opened {
		fmt.Fprintln(os.Stderr, "opened your browser to create an account.")
	}
	fmt.Fprintf(os.Stderr, "\n  %s\n\n", url)
	fmt.Fprintln(os.Stderr, "once you have signed up, create an API key on the API page and paste it here.")

	// Falls through to the SAME device flow login uses, so a brand-new user
	// finishes in the browser they already have open rather than being asked to
	// find a key and paste it back. One path writes a credential.
	fmt.Fprintln(os.Stderr, "once you have an account, come back here to finish.")
	return browserLogin(fSession, fNoBrowser)
}

// cmdLoginAfterSignup prompts for and stores a key.
func cmdLoginAfterSignup() error {
	key, err := promptKey()
	if err != nil {
		return err
	}
	if key == "" {
		fmt.Fprintf(os.Stderr, "no key given; run `%s login` when you have one.\n", progBase)
		return nil
	}
	if err := checkKey(key, fBaseURL); err != nil {
		return err
	}
	name := fSession
	if name == "" {
		name = defaultSession
	}
	gConfig.Sessions[name] = &Session{Key: key, BaseURL: fBaseURL, Created: time.Now()}
	gConfig.Active = name
	if err := SaveConfig(gConfig); err != nil {
		return err
	}
	fmt.Printf("stored key %s in session %q, now active\n", maskKey(key), name)
	return nil
}
