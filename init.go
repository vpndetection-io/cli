package main

import (
	"fmt"
	"os"
)

// The config is loaded before any command runs, so every command can read
// gConfig without each one remembering to.
//
// A failure is a warning rather than a fatal error: an unreadable config is a
// reason to run unauthenticated, not a reason for `vpndetection 1.1.1.1` to
// stop working. Nothing writes over it either: SaveConfig refuses to replace a
// file LoadConfig could not read.
func init() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: %v\n", err)
		fmt.Fprintln(os.Stderr, "warn: continuing with defaults; stored sessions are not available")
		gConfig = NewConfig()
		return
	}
	gConfig = cfg
}
