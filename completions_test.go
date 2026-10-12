package main

import (
	"io"
	"os"
	"os/exec"
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
		eachFlag(c.cmd, func(flag string, _ complete.Predictor) { offered[flag] = true })
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

// eachFlag calls fn with every flag a command and every subcommand under it
// offer, and the flag's predictor.
func eachFlag(cmd *complete.Command, fn func(flag string, p complete.Predictor)) {
	if cmd == nil {
		return
	}
	for flag, p := range cmd.Flags {
		fn(flag, p)
	}
	for _, sub := range cmd.Sub {
		eachFlag(sub, fn)
	}
}

// Completion and commandArg agree on which flags take a value. commandArg skips
// the argument after each flag valueFlags lists, and completion offers the
// level's options again after a flag declared with predict.Nothing, so a flag
// declared the wrong way round offers flags where its value goes, or nothing
// where the next flag goes.
func TestCompletionKnowsWhichFlagsTakeAValue(t *testing.T) {
	handleCompletions()
	eachFlag(completions, func(flag string, p complete.Predictor) {
		// predict.Nothing is a nil PredictFunc.
		f, isFunc := p.(complete.PredictFunc)
		takesValue := p != nil && (!isFunc || f != nil)
		if takesValue != valueFlags[flag] {
			t.Errorf("%s: completion takes a value after it: %t, valueFlags lists it: %t",
				flag, takesValue, valueFlags[flag])
		}
	})
}

// After a flag that takes no value, completion offers what it offered before
// the flag. Up to libgo-complete 1.0.1 it offered nothing after --json, and bash
// fell back to file names.
func TestCompletionAfterAFlag(t *testing.T) {
	for _, c := range []struct{ before, after string }{
		{"vpndetection ", "vpndetection --nocache "},
		{"vpndetection bulk ", "vpndetection bulk --json "},
		{"vpndetection db download ", "vpndetection db download --stdout "},
	} {
		want := completeLine(t, c.before)
		if want == "" {
			t.Fatalf("%q completes to nothing", c.before)
		}
		if got := completeLine(t, c.after); got != want {
			t.Errorf("%q completes to\n%s\nwant what %q does\n%s", c.after, got, c.before, want)
		}
	}
	// A flag that takes a value is offered its values instead.
	if got := completeLine(t, "vpndetection bulk --format "); got != "csv\njson\njsonl\npretty\nyaml\n" {
		t.Errorf("--format completes to %q, want the five formats", got)
	}
}

// completeLine is what this binary prints when a shell asks it to complete line.
func completeLine(t *testing.T, line string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self)
	cmd.Env = append(os.Environ(), "VPNDETECTION_TEST_RUN_MAIN=1", "COMP_LINE="+line,
		"XDG_CONFIG_HOME="+t.TempDir())
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("completing %q: %v", line, err)
	}
	return string(out)
}
