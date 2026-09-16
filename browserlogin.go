package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/browser"
)

// browserLogin runs the device flow and stores what comes back.
//
// Shared by `login` and `signup` so there is exactly one path that writes a
// credential, exactly as the pasted-key path already funnels through one.
func browserLogin(sessionName string, noBrowser bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	dev, err := startDeviceAuth(ctx)
	if err != nil {
		return fmt.Errorf("could not start sign-in: %w", err)
	}

	target := dev.VerificationURI
	if complete := dev.VerificationURIComplete; complete != nil && *complete != "" {
		target = *complete
	}

	opened := false
	if !noBrowser {
		// Errors ignored on purpose: a headless machine has no browser, and the
		// printed instructions below are the answer in both cases.
		opened = browser.OpenURL(target) == nil
	}

	// Printed even when the browser opened. The code is what the user confirms
	// on screen, so hiding it makes the page look like it is asking for
	// something they were never given - and if the browser opened on the WRONG
	// machine, which happens over SSH with X forwarding, this is the only way
	// through.
	//
	// stderr, not stdout: this is a prompt, and someone redirecting stdout is
	// capturing output, not instructions.
	if opened {
		fmt.Fprintln(os.Stderr, "opened your browser to finish signing in.")
	} else {
		fmt.Fprintln(os.Stderr, "open this page to finish signing in:")
	}
	fmt.Fprintf(os.Stderr, "\n  %s\n\n", dev.VerificationURI)
	fmt.Fprintf(os.Stderr, "and confirm this code:  %s\n\n", dev.UserCode)
	fmt.Fprintln(os.Stderr, "waiting for you to approve...")

	tok, err := pollForToken(ctx, dev)
	if err != nil {
		return err
	}

	if tok.Apikey == nil || *tok.Apikey == "" {
		// The flow succeeded but no key came back. The honest causes are that
		// the user picked nothing, or that the key they picked predates this
		// brand enabling key retrieval and so was never stored recoverably.
		// Guessing between them would be worse than saying what to do.
		return fmt.Errorf(
			"signed in, but no API key was handed over.\n"+
				"pick a key in the browser, or create one in the console and run `%s login`",
			progBase,
		)
	}

	name := sessionName
	if name == "" {
		name = defaultSession
	}
	existing := gConfig.Sessions[name]
	key := *tok.Apikey
	session := &Session{Key: key, BaseURL: fBaseURL, Created: time.Now()}
	if existing != nil {
		session.Created = existing.Created
		if fBaseURL == "" {
			session.BaseURL = existing.BaseURL
		}
	}
	if tok.RefreshToken != nil {
		session.RefreshToken = *tok.RefreshToken
	}
	if tok.ApikeyID != nil {
		session.APIKeyID = *tok.ApikeyID
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
