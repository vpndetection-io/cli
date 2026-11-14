package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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

// Login checks a key with a lookup, which any key may make, so a key without
// the db.download scope is stored, and a key the API refuses is not.
func TestLoginChecksAKeyWithALookup(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	var refuse atomic.Bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if refuse.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"error":"invalid API key"}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ip": strings.TrimPrefix(r.URL.Path, "/"), "is_vpn": false})
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedNoCache := gConfig, fNoCache
	t.Cleanup(func() { gConfig, fNoCache = savedConfig, savedNoCache })
	gConfig, fNoCache = NewConfig(), true

	if err := checkKey("k_lookups_only", api.URL); err != nil {
		t.Errorf("a key the lookup took was refused: %v", err)
	}
	refuse.Store(true)
	if err := checkKey("k_refused", api.URL); err == nil || !strings.Contains(err.Error(), "was not accepted") {
		t.Errorf("a key the lookup refused: %v, want it refused", err)
	}
	want := []string{"GET /1.1.1.1 Bearer k_lookups_only", "GET /1.1.1.1 Bearer k_refused"}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("requested %v, want %v", seen, want)
	}
}
