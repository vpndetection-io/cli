package main

import (
	"encoding/json"
	"fmt"
	"io"
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

// Login checks a key with a lookup, so a key without the db.download scope is
// stored, and a key the API refuses is not.
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

// A key the lookup refuses as forbidden is stored when it still lists the
// databases, as one holding db.download but not ip.lookup does. One refused for
// its address fails the listing too, and over plain HTTP the listing is never
// asked, since its host would redirect it to HTTPS and answer.
func TestCheckKeyStoresAKeyThatOnlyDownloads(t *testing.T) {
	const (
		scope    = "API key lacks the ip.lookup scope; add IP lookup to it in the console"
		address  = "source address not allowed for this API key"
		invalid  = "invalid API key"
		overHTTP = "API key sent over plain HTTP; use https://"
		catalog  = `{"databases":[]}`
		refused  = `{"rc":"UNAUTHORIZED"}`
	)
	cases := []struct {
		name         string
		https        bool
		lookupStatus int
		lookupError  string
		listStatus   int
		listBody     string
		stored       bool
		listed       bool
	}{
		{"db.download without ip.lookup", true, 403, scope, 200, catalog, true, true},
		{"neither scope", true, 403, scope, 401, refused, false, true},
		{"address not allowed", true, 403, address, 401, refused, false, true},
		{"invalid key", true, 401, invalid, 200, catalog, false, false},
		{"plain HTTP", false, 403, overHTTP, 200, catalog, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var listed atomic.Bool
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/1.1.1.1":
					w.WriteHeader(c.lookupStatus)
					_, _ = io.WriteString(w, `{"error":"`+c.lookupError+`"}`)
				case "/api/v1/database/list":
					listed.Store(true)
					w.WriteHeader(c.listStatus)
					_, _ = io.WriteString(w, c.listBody)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			})
			var api *httptest.Server
			if c.https {
				api = httptest.NewTLSServer(handler)
				savedTransport := http.DefaultTransport
				http.DefaultTransport = api.Client().Transport
				t.Cleanup(func() { http.DefaultTransport = savedTransport })
			} else {
				api = httptest.NewServer(handler)
			}
			t.Cleanup(api.Close)

			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("VPNDETECTION_API_KEY", "")
			t.Setenv("VPNDETECTION_BASE_URL", "")
			t.Setenv("VPNDETECTION_SESSION", "")
			savedConfig, savedNoCache, savedRetries, savedStderr := gConfig, fNoCache, fRetries, os.Stderr
			t.Cleanup(func() {
				gConfig, fNoCache, fRetries, os.Stderr = savedConfig, savedNoCache, savedRetries, savedStderr
			})
			gConfig, fNoCache, fRetries = NewConfig(), true, 0
			stderr, err := os.CreateTemp(t.TempDir(), "stderr")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = stderr.Close() })
			os.Stderr = stderr

			err = checkKey("k-test", api.URL)
			os.Stderr = savedStderr
			warned, readErr := os.ReadFile(stderr.Name())
			if readErr != nil {
				t.Fatal(readErr)
			}

			if listed.Load() != c.listed {
				t.Errorf("listing requested: %v, want %v", listed.Load(), c.listed)
			}
			if !c.stored {
				if err == nil || !strings.Contains(err.Error(), "the key was not accepted") ||
					!strings.Contains(err.Error(), c.lookupError) {
					t.Errorf("checkKey = %v, want the lookup's refusal, %q", err, c.lookupError)
				}
				return
			}
			if err != nil {
				t.Fatalf("checkKey = %v, want the key stored", err)
			}
			if got := string(warned); !strings.Contains(got, "cannot look up") || !strings.Contains(got, scope) {
				t.Errorf("warned %q, want it to say the key cannot look up, and the API's reason", got)
			}
		})
	}
}
