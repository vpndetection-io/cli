package main

import (
	"io"
	"strings"
	"testing"

	complete "github.com/mslmio/libgo-complete"
)

// Every flag a command's help lists, its completion offers. The two are written
// apart and nothing fails when they disagree: database's help listed --retries
// from 1.6.0, and its completion did not.
func TestCompletionOffersEveryFlagTheHelpLists(t *testing.T) {
	// handleCompletions gives the top level the lookup's flags, and returns when
	// no shell is asking.
	handleCompletions()
	cases := []struct {
		name string
		help func()
		cmd  *complete.Command
	}{
		// The top level alone: a subcommand's flags are not offered there.
		{"default", printHelpDefault, &complete.Command{Flags: completions.Flags}},
		{"bulk", printHelpBulk, completions.Sub["bulk"]},
		{"myip", printHelpMyIP, completions.Sub["myip"]},
		{"whoami", printHelpWhoami, completions.Sub["whoami"]},
		{"database", printHelpDatabase, completions.Sub["database"]},
		{"login", printHelpLogin, completions.Sub["login"]},
		{"logout", printHelpLogout, completions.Sub["logout"]},
		{"signup", printHelpSignup, completions.Sub["signup"]},
		{"session", printHelpSession, completions.Sub["session"]},
		{"cache", printHelpCache, completions.Sub["cache"]},
		{"config", printHelpConfig, completions.Sub["config"]},
		{"completion", printHelpCompletion, completions.Sub["completion"]},
	}
	listed := 0
	for _, c := range cases {
		file, _ := captureStdout(t, func() error {
			c.help()
			return nil
		})
		help, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		offered := map[string]bool{}
		collectFlags(c.cmd, offered)
		for _, flag := range helpFlags(string(help)) {
			listed++
			// Every command takes these, and completion offers them only beside
			// the lookup's flags.
			if flag == "--help" || flag == "-h" {
				continue
			}
			if !offered[flag] {
				t.Errorf("%s: the help lists %s, and completion does not offer it", c.name, flag)
			}
		}
	}
	if listed == 0 {
		t.Fatal("no help listed a flag, so the Options sections are no longer being read")
	}
}

// helpFlags is every spelling of every flag a help's Options section lists.
func helpFlags(help string) []string {
	_, opts, _ := strings.Cut(help, "\nOptions:\n")
	var flags []string
	for _, line := range strings.Split(opts, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-") {
			continue
		}
		// A format's description sits on its flag's line, two spaces on.
		line, _, _ = strings.Cut(line, "  ")
		for _, spelling := range strings.Split(line, ", ") {
			flags = append(flags, strings.Fields(spelling)[0])
		}
	}
	return flags
}

// collectFlags adds the flags a command and every subcommand under it offer.
func collectFlags(cmd *complete.Command, into map[string]bool) {
	if cmd == nil {
		return
	}
	for flag := range cmd.Flags {
		into[flag] = true
	}
	for _, sub := range cmd.Sub {
		collectFlags(sub, into)
	}
}
