package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Browser-based sign-in, via the OAuth 2.0 device authorization grant
// (RFC 8628).
//
// WHY THE DEVICE FLOW AND NOT A LOOPBACK REDIRECT: the other option for a
// command-line tool is to open a browser at a redirect pointing back to
// 127.0.0.1 on a port this process is listening on (RFC 8252). That is slightly
// nicer on a desktop and useless everywhere else - over SSH, in a container, on
// a build agent - which is a large share of where this tool actually runs.
// Shipping both would mean two flows to keep correct and one that fails exactly
// where it is hardest to debug. The device flow still opens a browser when there
// is one, so the desktop case loses almost nothing.
//
// What comes back is an ordinary API KEY, which is what every later request
// uses. The OAuth tokens are kept only so `whoami` can name the human and
// `logout` can revoke server-side rather than just deleting a local file.

// clientID identifies this program to the authorization server.
//
// Public, hardcoded and not a secret - which is the normal shape for a client
// that ships as a binary anyone can read. It is why the flow requires PKCE and
// why the server issues nothing on the strength of this value alone.
const clientID = "vpndetection-cli"

// The scopes login asks for. Narrow on purpose: everything here is read-only
// except for the one act of handing over a key the user themselves selects.
// Notably absent is anything that could CREATE a credential.
const loginScopes = "account.read apikeys.read apikeys.reveal"

// authBaseURL derives the authorization server from the API host.
//
// Same derivation as the console URL in cmd_signup.go and for the same reason:
// a session pointed at another deployment must authenticate against THAT
// deployment, never silently against production. Anything unrecognised falls
// back to production rather than guessing.
func authBaseURL() string {
	const prod = "https://auth.vpndetection.io"
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
	return "https://auth" + rest
}

type deviceAuth struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	CompleteURI     string `json:"verification_uri_complete"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`

	// Not part of any OAuth RFC, and namespaced so it cannot be mistaken for
	// one. This is the whole point of the flow for this program.
	APIKey   string `json:"mslm:apikey"`
	APIKeyID string `json:"mslm:apikey_id"`

	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// startDeviceAuth asks the authorization server to begin a flow.
func startDeviceAuth(ctx context.Context) (*deviceAuth, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", loginScopes)

	var out deviceAuth
	if err := postForm(ctx, authBaseURL()+"/oauth/device_authorization", form, &out); err != nil {
		return nil, err
	}
	if out.DeviceCode == "" || out.UserCode == "" {
		return nil, errors.New("the authorization server returned an incomplete response")
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	return &out, nil
}

// pollForToken waits for the human to approve, then returns the tokens.
//
// The interval is the server's, and `slow_down` widens it permanently rather
// than for one tick - that is what RFC 8628 asks for, and a client that resets
// its interval after a single slow_down just gets told again.
func pollForToken(ctx context.Context, dev *deviceAuth) (*tokenResp, error) {
	interval := time.Duration(dev.Interval) * time.Second
	deadline := time.Now().Add(time.Duration(dev.ExpiresIn) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		if time.Now().After(deadline) {
			return nil, errors.New("the code expired before it was approved; run the command again")
		}

		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("device_code", dev.DeviceCode)
		form.Set("client_id", clientID)

		var out tokenResp
		if err := postForm(ctx, authBaseURL()+"/oauth/token", form, &out); err != nil {
			return nil, err
		}

		switch out.Error {
		case "":
			return &out, nil
		case "authorization_pending":
			// The normal case for most of this loop: nobody has clicked yet.
		case "slow_down":
			interval += 5 * time.Second
		case "access_denied":
			return nil, errors.New("the request was denied in the browser")
		case "expired_token":
			return nil, errors.New("the code expired before it was approved; run the command again")
		default:
			if out.ErrorDescription != "" {
				return nil, fmt.Errorf("%s: %s", out.Error, out.ErrorDescription)
			}
			return nil, errors.New(out.Error)
		}
	}
}

// revokeToken tells the server to forget a credential.
//
// Best-effort by design, and the caller ignores the error: `logout` must always
// succeed locally. A machine that is offline, or whose token has already
// expired, still needs its stored credential gone - refusing to log out because
// the network is down would be the wrong answer to "remove this from my laptop".
func revokeToken(ctx context.Context, token string) error {
	form := url.Values{}
	form.Set("token", token)
	form.Set("client_id", clientID)
	var out map[string]any
	return postForm(ctx, authBaseURL()+"/oauth/revoke", form, &out)
}

func postForm(ctx context.Context, endpoint string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Bounded because this is an unauthenticated endpoint on the open internet:
	// an unbounded ReadAll here would let a broken or hostile responder decide
	// how much memory this process uses.
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}

	// A 4xx from the token endpoint carries a MEANINGFUL body - that is where
	// `authorization_pending` lives - so the status alone must not short-circuit
	// parsing. Only a response that is not JSON at all is a transport failure.
	if err := json.Unmarshal(body, out); err != nil {
		if res.StatusCode >= 400 {
			return fmt.Errorf("the authorization server returned %s", res.Status)
		}
		return fmt.Errorf("could not read the authorization server's response: %w", err)
	}
	return nil
}
