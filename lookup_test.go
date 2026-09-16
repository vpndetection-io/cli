package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/vpndetection-io/cli/lib"
)

// Chunking to the batch endpoint's 1000 belongs to the SDK, so the bulk path
// must hand it everything it read rather than capping or splitting on its own:
// 2500 addresses are three requests to POST /batch and one answer each.
func TestBulkIsNotCappedAtTheBatchEndpointsThousand(t *testing.T) {
	var batches atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/batch" {
			t.Errorf("requested %s %s, want only POST /batch", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		batches.Add(1)
		var in struct {
			IPs []string `json:"ips"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, `{"error":"unreadable body"}`, http.StatusBadRequest)
			return
		}
		results := make(map[string]any, len(in.IPs))
		for _, ip := range in.IPs {
			results[ip] = map[string]any{"ip": ip, "is_vpn": false}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results, "errors": map[string]any{}})
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache := gConfig, fBaseURL, fNoCache
	t.Cleanup(func() { gConfig, fBaseURL, fNoCache = savedConfig, savedBaseURL, savedNoCache })
	gConfig, fBaseURL, fNoCache = NewConfig(), api.URL, true

	addrs := make([]string, 2500)
	for i := range addrs {
		addrs[i] = fmt.Sprintf("9.1.%d.%d", i/256, i%256)
	}
	out := captureStdout(t, func() error {
		return runLookup(t.Context(), addrs, lookupOpts{format: lib.FormatJSONL, forceBulk: true})
	})

	if n := batches.Load(); n != 3 {
		t.Errorf("sent %d POST /batch request(s), want 3 for %d addresses", n, len(addrs))
	}
	answered := map[string]bool{}
	lines := bufio.NewScanner(out)
	for lines.Scan() {
		var answer struct {
			IP    string `json:"ip"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(lines.Bytes(), &answer); err != nil {
			t.Fatalf("unreadable output line %q: %v", lines.Text(), err)
		}
		if answer.Error != "" {
			t.Errorf("%s: %s", answer.IP, answer.Error)
		}
		if answered[answer.IP] {
			t.Errorf("%s was answered twice", answer.IP)
		}
		answered[answer.IP] = true
	}
	for _, ip := range addrs {
		if !answered[ip] {
			t.Fatalf("%s has no answer; %d of %d were answered", ip, len(answered), len(addrs))
		}
	}
	if len(answered) != len(addrs) {
		t.Errorf("answered %d address(es), want %d", len(answered), len(addrs))
	}
}

// captureStdout runs fn with os.Stdout pointed at a file, and returns that file
// rewound for reading.
func captureStdout(t *testing.T, fn func() error) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })

	saved := os.Stdout
	os.Stdout = file
	err = fn()
	os.Stdout = saved
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	return file
}
