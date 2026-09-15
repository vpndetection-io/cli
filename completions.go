package main

import (
	complete "github.com/mslmio/libgo-complete"
	"github.com/mslmio/libgo-complete/predict"
	"github.com/vpndetection-io/cli/lib"
)

// formats is every --format value, offered wherever one is taken.
var formats = predict.Set([]string{"pretty", "json", "jsonl", "csv", "yaml"})

// outputFlags are the ones every lookup command shares.
func outputFlags() map[string]complete.Predictor {
	return map[string]complete.Predictor{
		"--field": predict.Set(lib.Columns), "-f": predict.Set(lib.Columns),
		"--format":      formats,
		"--json":        predict.Nothing,
		"-j":            predict.Nothing,
		"--jsonl":       predict.Nothing,
		"--csv":         predict.Nothing,
		"-c":            predict.Nothing,
		"--yaml":        predict.Nothing,
		"-y":            predict.Nothing,
		"--pretty":      predict.Nothing,
		"-p":            predict.Nothing,
		"--show-absent": predict.Nothing,
		"--nocolor":     predict.Nothing,
		"--nocache":     predict.Nothing,
		"--concurrency": predict.Something,
		"--retries":     predict.Something,
		"--key":         predict.Something,
		"-k":            predict.Something,
		"--session":     sessionNames(),
		"--base-url":    predict.Something,
		"--help":        predict.Nothing,
		"-h":            predict.Nothing,
	}
}

// sessionNames offers the sessions this machine actually holds, which is the
// completion that saves the most typing and the one a static list cannot give.
func sessionNames() complete.Predictor {
	return predict.Func(func(string) []string { return gConfig.SessionNames() })
}

var completions = &complete.Command{
	Sub: map[string]*complete.Command{
		"bulk":     {Flags: outputFlags()},
		"myip":     {Flags: outputFlags()},
		"database": databaseCompletions,
		"db":       databaseCompletions,
		"login":    loginCompletions,
		"init":     loginCompletions,
		"logout": {Flags: map[string]complete.Predictor{
			"--session": sessionNames(),
			"--all":     predict.Nothing,
		}},
		"signup": {Flags: map[string]complete.Predictor{"--no-browser": predict.Nothing}},
		"session": {Sub: map[string]*complete.Command{
			"list":   {},
			"use":    {Args: sessionNames()},
			"show":   {Args: sessionNames()},
			"rename": {Args: sessionNames()},
			"rm":     {Args: sessionNames()},
		}},
		"whoami":      {},
		"entitlement": {},
		"cache":       {Args: predict.Set([]string{"info", "clear"})},
		"config": {Args: predict.Set([]string{
			"cache=enable", "cache=disable", "cache_ttl=", "format=",
			"concurrency=", "retries=",
		})},
		"completion": {Args: predict.Set([]string{"install", "uninstall", "bash", "zsh", "fish"})},
		"version":    {},
	},
	Flags: map[string]complete.Predictor{
		"--version": predict.Nothing,
		"--vsn":     predict.Nothing,
		"-v":        predict.Nothing,
		"--help":    predict.Nothing,
		"-h":        predict.Nothing,
	},
}

// databaseCompletions is shared by `database` and its `db` alias, so the two
// cannot drift into offering different subcommands.
var databaseCompletions = &complete.Command{
	Sub: map[string]*complete.Command{
		"list":      {},
		"metadata":  {},
		"checksum":  {Flags: map[string]complete.Predictor{"--format": predict.Set([]string{"csvgz", "mmdb"})}},
		"url":       {Flags: map[string]complete.Predictor{"--format": predict.Set([]string{"csvgz", "mmdb"})}},
		"downloads": {Flags: map[string]complete.Predictor{"--limit": predict.Something}},
		"download": {Flags: map[string]complete.Predictor{
			"--format":    predict.Set([]string{"csvgz", "mmdb"}),
			"--stdout":    predict.Nothing,
			"--no-verify": predict.Nothing,
		}},
	},
}

// loginCompletions is shared by `login` and its `init` alias.
var loginCompletions = &complete.Command{
	Flags: map[string]complete.Predictor{
		"--session":  sessionNames(),
		"--base-url": predict.Something,
		"--key":      predict.Something,
		"-k":         predict.Something,
		"--no-check": predict.Nothing,
	},
}

// handleCompletions answers a shell completion request, if this is one.
func handleCompletions() {
	// Also the top-level flag set, so completing at the very start offers the
	// output options an address lookup takes.
	for flag, p := range outputFlags() {
		if _, taken := completions.Flags[flag]; !taken {
			completions.Flags[flag] = p
		}
	}
	completions.Complete(progBase)
}
