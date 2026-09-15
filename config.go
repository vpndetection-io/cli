package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// gConfig is the loaded config, read once at startup by init.
var gConfig Config

// Config is everything the CLI remembers between runs.
//
// Sessions are named credentials rather than one token, so a machine that talks
// to more than one organization, or to more than one deployment of the API,
// does not have to re-authenticate to switch. Exactly one is active at a time.
type Config struct {
	// Version of this file's shape, so a future change can migrate rather than
	// guess.
	Version int `json:"version"`

	// Active names the session in use. Empty means unauthenticated, which is a
	// working state: the API answers `ip` and `is_vpn` with no key at all.
	Active string `json:"active"`

	// Sessions are the stored credentials, keyed by name.
	Sessions map[string]*Session `json:"sessions"`

	// CacheEnabled controls the on-disk lookup cache.
	CacheEnabled bool `json:"cache_enabled"`

	// CacheTTL is how long a cached answer stays good, as a duration string.
	CacheTTL string `json:"cache_ttl"`

	// Format is the default output format for a single lookup.
	Format string `json:"format"`

	// Concurrency and Retries default what the client is built with.
	Concurrency int `json:"concurrency"`
	Retries     int `json:"retries"`
}

// Session is one stored credential.
type Session struct {
	// Key is the API key. Stored in plain text in a 0600 file, the same way
	// every CLI credential on a developer machine is; the alternative is a
	// keyring this would have to carry per platform.
	Key string `json:"key"`

	// BaseURL overrides the API this session talks to, which is what lets a
	// session on another deployment sit beside a production one.
	BaseURL string `json:"base_url,omitempty"`

	Created  time.Time `json:"created"`
	LastUsed time.Time `json:"last_used,omitempty"`

	// RefreshToken is present only for a session created by `login` in a
	// browser. It is NOT used for lookups - the API key above is - and exists
	// so `logout` can revoke server-side instead of only deleting this file,
	// which is the difference between signing out and forgetting.
	RefreshToken string `json:"refresh_token,omitempty"`

	// APIKeyID names which key this session holds, so the console's key list
	// and this machine can be matched up without ever comparing secrets.
	APIKeyID string `json:"apikey_id,omitempty"`
}

const (
	configVersion      = 1
	defaultSession     = "default"
	defaultCacheTTL    = "1h"
	defaultFormat      = "pretty"
	defaultConcurrency = 8
	defaultRetries     = 2
)

// NewConfig is the config a first run gets.
func NewConfig() Config {
	return Config{
		Version:      configVersion,
		Sessions:     map[string]*Session{},
		CacheEnabled: true,
		CacheTTL:     defaultCacheTTL,
		Format:       defaultFormat,
		Concurrency:  defaultConcurrency,
		Retries:      defaultRetries,
	}
}

// ConfigDir is where the config and cache live, created if missing.
//
// 0700 because the config file holds API keys; a 0755 directory would leave
// them listable by every other account on a shared machine.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "vpndetection")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ConfigPath is the config file.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig reads the config, creating a default one if there is none.
//
// A file that cannot be parsed is NOT silently replaced: it holds credentials
// the user cannot get back, and overwriting it to recover from a stray edit
// would lose them. The error names the path so it can be fixed by hand.
func LoadConfig() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := NewConfig()
		return cfg, SaveConfig(cfg)
	}
	if err != nil {
		return Config{}, err
	}

	cfg := NewConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	if cfg.Sessions == nil {
		cfg.Sessions = map[string]*Session{}
	}
	return cfg, nil
}

// SaveConfig writes the config back, readable only by its owner.
func SaveConfig(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	cfg.Version = configVersion
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	// Written to a neighbouring file and renamed, so an interrupted write
	// cannot leave a truncated file - which here means losing every stored key.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// SessionNames are the stored sessions, sorted.
func (c Config) SessionNames() []string {
	names := make([]string, 0, len(c.Sessions))
	for name := range c.Sessions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ActiveSession is the session in use, or nil when running unauthenticated.
//
// Precedence is --session, then VPNDETECTION_SESSION, then whatever the config
// says is active. The flag wins so a one-off command can borrow another
// credential without changing anything on disk.
func (c Config) ActiveSession() *Session {
	name := c.ActiveSessionName()
	if name == "" {
		return nil
	}
	return c.Sessions[name]
}

// ActiveSessionName resolves which session name is in play.
func (c Config) ActiveSessionName() string {
	if fSession != "" {
		return fSession
	}
	if env := os.Getenv("VPNDETECTION_SESSION"); env != "" {
		return env
	}
	return c.Active
}

// ResolveKey is the API key to present, and where it came from.
//
// The order is deliberate: an explicit flag beats the environment, which beats
// stored state, and no key at all is a valid answer rather than an error - the
// free tier needs none. A command that genuinely requires one says so itself,
// with a message naming how to get one.
func (c Config) ResolveKey() (key string, source string) {
	if fKey != "" {
		return fKey, "--key"
	}
	if env := os.Getenv("VPNDETECTION_API_KEY"); env != "" {
		return env, "VPNDETECTION_API_KEY"
	}
	if s := c.ActiveSession(); s != nil && s.Key != "" {
		return s.Key, "session " + c.ActiveSessionName()
	}
	return "", "unauthenticated"
}

// ResolveBaseURL is the API to talk to. An empty string means the SDK default.
func (c Config) ResolveBaseURL() string {
	if fBaseURL != "" {
		return fBaseURL
	}
	if env := os.Getenv("VPNDETECTION_BASE_URL"); env != "" {
		return env
	}
	if s := c.ActiveSession(); s != nil {
		return s.BaseURL
	}
	return ""
}

// keyFingerprint identifies a key without storing or showing it.
//
// Used to scope the cache and to show which credential is in play. Truncated
// because it is an identifier, not a digest anyone verifies.
func keyFingerprint(key string) string {
	if key == "" {
		return "none"
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:12]
}

// maskKey shows enough of a key to recognise it and not enough to use it.
func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

// cacheTTL is the configured TTL, falling back to the default when the stored
// value is unparseable rather than failing every lookup over it.
func (c Config) cacheTTL() time.Duration {
	if c.CacheTTL != "" {
		if d, err := time.ParseDuration(c.CacheTTL); err == nil && d > 0 {
			return d
		}
	}
	d, _ := time.ParseDuration(defaultCacheTTL)
	return d
}
