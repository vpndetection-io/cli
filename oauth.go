package main

import (
	"context"
	"errors"
	"strings"

	vpndetection "github.com/vpndetection-io/sdk-go/v5"
)

// Browser-based sign-in, via the OAuth 2.0 device authorization grant
// (RFC 8628), through the SDK's client.Oauth.
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
// uses. The OAuth tokens are kept only so `logout` can revoke server-side
// rather than just deleting a local file.

// clientID identifies this program to the authorization server. Public,
// hardcoded and not a secret: it ships in a binary anyone can read, and a person
// still approves every sign-in in the browser.
const clientID = "vpndetection-cli"

// The scopes login asks for. Narrow on purpose: everything here is read-only
// except for the one act of handing over a key the user themselves selects.
// Notably absent is anything that could CREATE a credential.
const loginScopes = "account.read apikeys.read apikeys.reveal"

// authBaseURL is where the authorization server lives.
//
// It is simply the API base URL, because the authorization server is mounted on
// PATHS of the API host (`/oauth/`, `/.well-known/`) rather than on a host of
// its own. That is the whole reason this function is three lines: there is no
// hostname to derive, so there is nothing here to get wrong.
//
// It also means a session pointed at another deployment authenticates against
// THAT deployment automatically, rather than depending on this program agreeing
// with the server about how to rewrite a host.
func authBaseURL() string {
	if base := gConfig.ResolveBaseURL(); base != "" {
		return strings.TrimSuffix(base, "/")
	}
	return vpndetection.DefaultBaseURL
}

// startDeviceAuth asks the authorization server to begin a flow.
func startDeviceAuth(ctx context.Context) (*vpndetection.DeviceAuthorization, error) {
	oauth, err := signInAPI()
	if err != nil {
		return nil, err
	}
	dev, err := oauth.DeviceAuthorization(ctx, clientID,
		vpndetection.DeviceAuthorizationOptions{Scope: loginScopes})
	var refused *vpndetection.OauthError
	if errors.As(err, &refused) && refused.ErrorCode == "slow_down" {
		return nil, errors.New("too many sign-in attempts from this address; wait a minute and try again")
	}
	return dev, err
}

// pollForToken waits for the human to approve, then returns the tokens. The SDK
// keeps the server's interval, widened for good by each slow_down.
func pollForToken(
	ctx context.Context, dev *vpndetection.DeviceAuthorization,
) (*vpndetection.TokenResponse, error) {
	oauth, err := signInAPI()
	if err != nil {
		return nil, err
	}
	tok, err := oauth.PollDeviceToken(ctx, clientID, dev)
	switch {
	case errors.Is(err, vpndetection.ErrOauthAccessDenied):
		return nil, errors.New("the request was denied in the browser")
	case errors.Is(err, vpndetection.ErrOauthExpiredToken):
		return nil, errors.New("the code expired before it was approved; run the command again")
	}
	return tok, err
}

// revokeToken tells the server to forget a credential.
//
// Best-effort by design, and the caller ignores the error: `logout` must always
// succeed locally. A machine that is offline, or whose token has already
// expired, still needs its stored credential gone - refusing to log out because
// the network is down would be the wrong answer to "remove this from my laptop".
func revokeToken(ctx context.Context, token string) error {
	oauth, err := signInAPI()
	if err != nil {
		return err
	}
	return oauth.Revoke(ctx, clientID, token)
}

// signInAPI is a keyless client aimed at this session's deployment. The SDK
// keeps any key off these requests regardless; there is simply none to give.
func signInAPI() (*vpndetection.OauthAPI, error) {
	client, err := vpndetection.New(
		vpndetection.WithBaseURL(authBaseURL()),
		vpndetection.WithRetries(resolveRetries()),
		vpndetection.WithoutCache(),
	)
	if err != nil {
		return nil, err
	}
	return client.Oauth, nil
}
