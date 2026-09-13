package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vpndetection-io/cli/lib"
)

func printHelpConfig() {
	fmt.Printf(
		`Usage: %[1]s config [list | <key>=<value>...]

Description:
  Read or change stored settings.

Examples:
  $ %[1]s config list
  $ %[1]s config cache=disable
  $ %[1]s config format=json cache_ttl=24h

Settings:
  cache=<enable | disable>
    whether answers are cached on disk.
  cache_ttl=<duration>
    how long a cached answer stays good, as 30m, 1h, 24h.
  format=<pretty | json | jsonl | csv | yaml>
    default output for a single address. Many addresses always default to json.
  concurrency=<n>
    in-flight requests during a bulk lookup.
  retries=<n>
    retries per request.

Options:
  --help, -h
    show help.

Credentials are not settings; see '%[1]s session'.
`, progBase)
}

func cmdConfig() error {
	globalFlags()
	args := parseSubFlags()

	// A bare `config` prints help rather than the settings. It is the shape
	// every other subcommand here has, and `config list` is one word away.
	if fHelp || len(args) == 0 {
		printHelpConfig()
		return nil
	}
	if len(args) == 1 && (args[0] == "list" || args[0] == "ls") {
		return configShow()
	}

	// Applied to a COPY, so a batch of settings with one bad value changes
	// nothing rather than applying up to the failure.
	next := gConfig
	for _, arg := range args {
		k, v, found := strings.Cut(arg, "=")
		if !found {
			return fmt.Errorf("expected <key>=<value>, got %q", arg)
		}
		if err := applySetting(&next, strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)); err != nil {
			return err
		}
	}
	if err := SaveConfig(next); err != nil {
		return err
	}
	gConfig = next
	return configShow()
}

// applySetting validates and applies one key=value.
func applySetting(cfg *Config, key, value string) error {
	switch key {
	case "cache":
		switch strings.ToLower(value) {
		case "enable", "enabled", "on", "true":
			cfg.CacheEnabled = true
		case "disable", "disabled", "off", "false":
			cfg.CacheEnabled = false
		default:
			return fmt.Errorf("cache must be enable or disable, got %q", value)
		}
	case "cache_ttl":
		d, err := time.ParseDuration(value)
		if err != nil || d <= 0 {
			return fmt.Errorf("cache_ttl must be a positive duration such as 1h, got %q", value)
		}
		cfg.CacheTTL = value
	case "format":
		format, err := lib.ParseFormat(value)
		if err != nil {
			return err
		}
		cfg.Format = string(format)
	case "concurrency":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("concurrency must be a positive number, got %q", value)
		}
		cfg.Concurrency = n
	case "retries":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("retries must be zero or more, got %q", value)
		}
		cfg.Retries = n
	default:
		return fmt.Errorf("%q is not a setting; `%s config --help` lists them", key, progBase)
	}
	return nil
}

func configShow() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	state := "enable"
	if !gConfig.CacheEnabled {
		state = "disable"
	}
	fmt.Printf("cache        %s\n", state)
	fmt.Printf("cache_ttl    %s\n", gConfig.cacheTTL())
	fmt.Printf("format       %s\n", gConfig.Format)
	fmt.Printf("concurrency  %d\n", resolveConcurrency())
	fmt.Printf("retries      %d\n", resolveRetries())
	fmt.Printf("\nstored in    %s\n", path)
	return nil
}
