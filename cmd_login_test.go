package main

import (
	"os"
	"strings"
	"testing"
)

// /dev/null is a character device, but not a terminal: a prompt must refuse it
// rather than fail inside the read.
func TestPromptKeyRefusesNullDevice(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	saved := os.Stdin
	os.Stdin = null
	defer func() { os.Stdin = saved }()

	_, err = promptKey()
	if err == nil || !strings.Contains(err.Error(), "no terminal to prompt on") {
		t.Fatalf("promptKey() with stdin from %s = %v, want the no-terminal refusal", os.DevNull, err)
	}
}
