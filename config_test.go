package main

import (
	"os"
	"testing"
	"time"
)

// The order is --key, then the environment, then the active session, then
// nothing - and "nothing" is a working state rather than an error, because the
// free tier needs no key.
func TestResolveKeyPrecedence(t *testing.T) {
	cfg := NewConfig()
	cfg.Sessions["work"] = &Session{Key: "from-session"}
	cfg.Active = "work"

	reset := func() {
		fKey, fSession = "", ""
		os.Unsetenv("VPNDETECTION_API_KEY")
		os.Unsetenv("VPNDETECTION_SESSION")
	}
	t.Cleanup(reset)

	reset()
	if key, src := cfg.ResolveKey(); key != "from-session" || src != "session work" {
		t.Errorf("stored session: got %q from %q", key, src)
	}

	reset()
	t.Setenv("VPNDETECTION_API_KEY", "from-env")
	if key, src := cfg.ResolveKey(); key != "from-env" || src != "VPNDETECTION_API_KEY" {
		t.Errorf("env beats session: got %q from %q", key, src)
	}

	fKey = "from-flag"
	if key, src := cfg.ResolveKey(); key != "from-flag" || src != "--key" {
		t.Errorf("flag beats env: got %q from %q", key, src)
	}

	reset()
	empty := NewConfig()
	if key, src := empty.ResolveKey(); key != "" || src != "unauthenticated" {
		t.Errorf("no credential: got %q from %q", key, src)
	}
}

func TestResolveSessionPrecedence(t *testing.T) {
	cfg := NewConfig()
	cfg.Sessions["a"] = &Session{Key: "ka"}
	cfg.Sessions["b"] = &Session{Key: "kb"}
	cfg.Active = "a"

	t.Cleanup(func() { fSession = "" })

	fSession = ""
	if got := cfg.ActiveSessionName(); got != "a" {
		t.Errorf("config: got %q", got)
	}
	t.Setenv("VPNDETECTION_SESSION", "b")
	if got := cfg.ActiveSessionName(); got != "b" {
		t.Errorf("env beats config: got %q", got)
	}
	fSession = "a"
	if got := cfg.ActiveSessionName(); got != "a" {
		t.Errorf("flag beats env: got %q", got)
	}
}

// The base URL comes from the SESSION, which is what lets a staging credential
// sit beside a production one without a flag on every command.
func TestResolveBaseURL(t *testing.T) {
	cfg := NewConfig()
	cfg.Sessions["staging"] = &Session{Key: "k", BaseURL: "https://api-staging.vpndetection.io"}
	cfg.Active = "staging"
	t.Cleanup(func() { fBaseURL = "" })

	if got := cfg.ResolveBaseURL(); got != "https://api-staging.vpndetection.io" {
		t.Errorf("got %q", got)
	}
	fBaseURL = "https://elsewhere.example"
	if got := cfg.ResolveBaseURL(); got != "https://elsewhere.example" {
		t.Errorf("flag should win: got %q", got)
	}
}

// A config file is written 0600 and its directory 0700, because it holds API
// keys in plain text.
func TestConfigFilePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := NewConfig()
	cfg.Sessions["a"] = &Session{Key: "secret", Created: time.Now()}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file is %o, want 600", perm)
	}
	confDir, _ := ConfigDir()
	dst, err := os.Stat(confDir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := dst.Mode().Perm(); perm != 0o700 {
		t.Errorf("config dir is %o, want 700", perm)
	}
}

// A config file that cannot be parsed must NOT be replaced with a default one:
// it holds credentials that cannot be recovered from anywhere else.
func TestLoadConfigRefusesToClobberGarbage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected an error rather than a silent overwrite")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "{not json" {
		t.Errorf("the file was rewritten: %q", body)
	}
}

func TestMaskKey(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"abc":               "***",
		"abcdefgh":          "********",
		"mk_1234567890abcd": "mk_1*********abcd",
	}
	for in, want := range cases {
		if got := maskKey(in); got != want {
			t.Errorf("maskKey(%q) = %q, want %q", in, got, want)
		}
	}
	// Whatever it shows, it must never be the key itself.
	if maskKey("mk_1234567890abcd") == "mk_1234567890abcd" {
		t.Error("the key leaked through the mask")
	}
}
