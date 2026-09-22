package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/term"
)

func printHelpLogin() {
	fmt.Printf(
		`Usage: %[1]s login [<opts>] [<key>]

Description:
  Sign in and store an API key so later commands use it.

  With no arguments this opens your browser, you confirm a short code, and you
  pick which of your API keys this machine should hold. Nothing is typed or
  pasted, and the key never reaches your shell history.

  Keys are stored in named sessions, so one machine can hold credentials for
  several organizations and switch between them with
  '%[1]s session use <name>'.

  Pass a key directly and it is stored as-is, which is what a CI job wants.

Examples:
  # Sign in through the browser (the usual way).
  $ %[1]s login

  # Paste a key instead, on a machine with no browser.
  $ %[1]s login --paste

  # A second credential, kept under its own name.
  $ %[1]s login --session work

  # A session pointed at another deployment of the API.
  $ %[1]s login --session acme --base-url https://api.example.com

Options:
  --paste
    prompt for a key instead of opening a browser.
  --no-browser
    print the sign-in URL instead of opening it.
  --session <name>
    name to store this key under. Default: %[2]s.
  --base-url <url>
    API this session talks to, for a non-production deployment.
  --key <key>, -k <key>
    the key, instead of being prompted. Your shell records this; prefer the
    prompt.
  --no-check
    skip verifying the key against the API before storing it.
  --help, -h
    show help.
`, progBase, defaultSession)
}

func cmdLogin() error {
	var fNoCheck, fPaste, fNoBrowser bool
	globalFlags()
	pflag.BoolVar(&fNoCheck, "no-check", false, "do not verify the key before storing it.")
	pflag.BoolVar(&fPaste, "paste", false, "prompt for a key instead of opening a browser.")
	pflag.BoolVar(&fNoBrowser, "no-browser", false, "print the sign-in URL instead of opening it.")
	args := parseSubFlags()

	if fHelp {
		printHelpLogin()
		return nil
	}
	if len(args) > 1 {
		return errors.New("expected at most one key")
	}

	// A key supplied explicitly is still honored, so a CI job and a script keep
	// working exactly as before. The browser flow is only the default for the
	// interactive case it was built for.
	if fKey == "" && len(args) == 0 && !fPaste {
		return browserLogin(fSession, fNoBrowser)
	}

	key := fKey
	if len(args) == 1 {
		if key != "" {
			// Silently preferring one would store a key the user did not mean
			// to store, which is the one mistake here that is hard to notice.
			return errors.New("the key was given twice, as --key and as an argument")
		}
		key = args[0]
	}
	if key == "" {
		var err error
		key, err = promptKey()
		if err != nil {
			return err
		}
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("no key given")
	}

	name := fSession
	if name == "" {
		name = defaultSession
	}

	if !fNoCheck {
		if err := checkKey(key, fBaseURL); err != nil {
			return err
		}
	}

	existing := gConfig.Sessions[name]
	session := &Session{Key: key, BaseURL: fBaseURL, Created: time.Now()}
	if existing != nil {
		// Re-logging in keeps the session's own base URL unless a new one was
		// given, so `login --session work` does not silently move it to
		// production.
		session.Created = existing.Created
		if fBaseURL == "" {
			session.BaseURL = existing.BaseURL
		}
	}
	gConfig.Sessions[name] = session
	gConfig.Active = name
	if err := SaveConfig(gConfig); err != nil {
		return err
	}

	verb := "stored"
	if existing != nil {
		verb = "replaced"
	}
	fmt.Printf("%s key %s in session %q, now active\n", verb, maskKey(key), name)
	return nil
}

// promptKey reads a key from the terminal without echoing it.
func promptKey() (string, error) {
	if !isTerminal(os.Stdin) {
		// Reading a key from a pipe would work and would also mean the key came
		// from somewhere that probably logged it. --key is the explicit way.
		return "", errors.New("no terminal to prompt on; pass --key or set VPNDETECTION_API_KEY")
	}
	fmt.Fprint(os.Stderr, "API key: ")
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// checkKey confirms the API accepts a key before it is stored.
//
// Worth a request: a mistyped key stored silently turns every later command
// into a 401, and the obvious explanation - "my plan lapsed" - is the wrong one.
func checkKey(key, baseURL string) error {
	saveKey, saveURL := fKey, fBaseURL
	fKey, fBaseURL = key, baseURL
	defer func() { fKey, fBaseURL = saveKey, saveURL }()

	client, err := NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// A public address, so the answer exercises the key rather than the bogon
	// short-circuit, which never reaches the network and so proves nothing.
	if _, err := client.api.Lookup(ctx, "1.1.1.1"); err != nil {
		return fmt.Errorf("the key was not accepted: %w\n(use --no-check to store it anyway)", explain(err))
	}
	return nil
}
