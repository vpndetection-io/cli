package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	iputil "github.com/mslmio/libgo-iputil"
	"github.com/vpndetection-io/cli/lib"
)

// Chunking to the batch endpoint's 1000 belongs to the SDK, so the bulk path
// must hand it everything it read rather than capping or splitting on its own:
// 2500 addresses are three requests to POST /batch and one answer each, with a
// key, which the batch needs.
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
	t.Setenv("VPNDETECTION_API_KEY", "test-key")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache := gConfig, fBaseURL, fNoCache
	t.Cleanup(func() { gConfig, fBaseURL, fNoCache = savedConfig, savedBaseURL, savedNoCache })
	gConfig, fBaseURL, fNoCache = NewConfig(), api.URL, true

	addrs := make([]string, 2500)
	for i := range addrs {
		addrs[i] = fmt.Sprintf("9.1.%d.%d", i/256, i%256)
	}
	out, err := captureStdout(t, func() error {
		return runLookup(t.Context(), addrs, lookupOpts{format: lib.FormatJSONL, forceBulk: true})
	})
	if err != nil {
		t.Fatal(err)
	}

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

// --user-agent goes with every address, keyed or not: in a batch's user_agents,
// and on each keyless GET as user_agent.
func TestTheVisitorsAgentGoesWithEveryAddress(t *testing.T) {
	const agent = "Googlebot/2.1 (+http://www.google.com/bot.html)"
	addrs := []string{"66.249.66.1", "146.70.116.130"}
	for _, key := range []string{"test-key", ""} {
		var mu sync.Mutex
		sent := map[string]string{}
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			mu.Lock()
			defer mu.Unlock()
			if r.Method == http.MethodPost {
				var in struct {
					IPs    []string          `json:"ips"`
					Agents map[string]string `json:"user_agents"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					http.Error(w, `{"error":"unreadable body"}`, http.StatusBadRequest)
					return
				}
				results := map[string]any{}
				for _, ip := range in.IPs {
					sent[ip] = in.Agents[ip]
					results[ip] = map[string]any{"ip": ip, "is_vpn": false}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"results": results, "errors": map[string]any{}})
				return
			}
			ip := strings.TrimPrefix(r.URL.Path, "/")
			sent[ip] = r.URL.Query().Get("user_agent")
			_ = json.NewEncoder(w).Encode(map[string]any{"ip": ip, "is_vpn": false})
		}))

		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("VPNDETECTION_API_KEY", key)
		t.Setenv("VPNDETECTION_SESSION", "")
		savedConfig, savedBaseURL, savedNoCache := gConfig, fBaseURL, fNoCache
		gConfig, fBaseURL, fNoCache = NewConfig(), api.URL, true
		_, err := captureStdout(t, func() error {
			return runLookup(t.Context(), addrs, lookupOpts{format: lib.FormatJSONL, agent: agent})
		})
		gConfig, fBaseURL, fNoCache = savedConfig, savedBaseURL, savedNoCache
		api.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, ip := range addrs {
			if sent[ip] != agent {
				t.Errorf("key %q: %s was sent agent %q, want %q", key, ip, sent[ip], agent)
			}
		}
	}
}

// --show-absent lists user_agent, as '-', for a lookup that sent an agent and was
// answered without a verdict, as a keyless one is, and never for one that sent none.
func TestShowAbsentListsTheVerdictOnlyForALookupSendingAnAgent(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ip": strings.TrimPrefix(r.URL.Path, "/"), "is_vpn": true})
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache := gConfig, fBaseURL, fNoCache
	t.Cleanup(func() { gConfig, fBaseURL, fNoCache = savedConfig, savedBaseURL, savedNoCache })
	gConfig, fBaseURL, fNoCache = NewConfig(), api.URL, true

	for _, agent := range []string{"Googlebot/2.1", ""} {
		out, err := captureStdout(t, func() error {
			opts := lookupOpts{format: lib.FormatPretty, showAbsent: true, agent: agent}
			return runLookup(t.Context(), []string{"146.70.116.130"}, opts)
		})
		if err != nil {
			t.Fatal(err)
		}
		block, err := io.ReadAll(out)
		if err != nil {
			t.Fatal(err)
		}
		if listed := strings.Contains(string(block), "user_agent.verdict"); listed != (agent != "") {
			t.Errorf("agent %q: user_agent.verdict listed is %v:\n%s", agent, listed, block)
		}
	}
}

// POST /batch needs a key, so without one each address is its own GET, answered
// all the same.
func TestLookupWithoutAKeyIsOneGetPerAddress(t *testing.T) {
	var gets atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "" {
			t.Errorf("requested %s %s with %q, want GET without a key", r.Method, r.URL.Path,
				r.Header.Get("Authorization"))
			http.NotFound(w, r)
			return
		}
		gets.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ip": strings.TrimPrefix(r.URL.Path, "/"), "is_vpn": false})
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache := gConfig, fBaseURL, fNoCache
	t.Cleanup(func() { gConfig, fBaseURL, fNoCache = savedConfig, savedBaseURL, savedNoCache })
	gConfig, fBaseURL, fNoCache = NewConfig(), api.URL, true

	addrs := make([]string, 25)
	for i := range addrs {
		addrs[i] = fmt.Sprintf("9.1.0.%d", i+1)
	}
	out, err := captureStdout(t, func() error {
		return runLookup(t.Context(), addrs, lookupOpts{format: lib.FormatJSONL})
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := int(gets.Load()); n != len(addrs) {
		t.Errorf("sent %d GET(s), want one for each of %d addresses", n, len(addrs))
	}
	lines := 0
	for scanner := bufio.NewScanner(out); scanner.Scan(); lines++ {
		if strings.Contains(scanner.Text(), `"error"`) {
			t.Errorf("unanswered: %s", scanner.Text())
		}
	}
	if lines != len(addrs) {
		t.Errorf("wrote %d answer(s), want %d", lines, len(addrs))
	}
}

// Without a key each address is its own lookup, and as many are in flight as
// --concurrency allows: no more, and no fewer, or a bulk run is as slow as one
// address at a time.
func TestLookupsWithoutAKeyRunAtTheConcurrency(t *testing.T) {
	const width = 4
	var inFlight, peak atomic.Int32
	var opened sync.Once
	release := make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		if n == width {
			opened.Do(func() { close(release) })
		}
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ip": strings.TrimPrefix(r.URL.Path, "/"), "is_vpn": false})
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache, savedConcurrency := gConfig, fBaseURL, fNoCache, fConcurrency
	t.Cleanup(func() {
		gConfig, fBaseURL, fNoCache, fConcurrency = savedConfig, savedBaseURL, savedNoCache, savedConcurrency
	})
	gConfig, fBaseURL, fNoCache, fConcurrency = NewConfig(), api.URL, true, width

	addrs := make([]string, 3*width)
	for i := range addrs {
		addrs[i] = fmt.Sprintf("9.1.0.%d", i+1)
	}
	if _, err := captureStdout(t, func() error {
		return runLookup(t.Context(), addrs, lookupOpts{format: lib.FormatJSONL})
	}); err != nil {
		t.Fatal(err)
	}
	if p := peak.Load(); p != width {
		t.Errorf("at most %d lookup(s) were in flight, want %d", p, width)
	}
}

// A free key, an org on no paid plan, may not batch more than one address: the API answers 402 for
// each, and the CLI looks each up on its own instead, answering every one.
func TestAFreeKeysRefusedBatchIsLookedUpOneByOne(t *testing.T) {
	var batches, gets atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			batches.Add(1)
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = fmt.Fprint(w, `{"error":"POST /batch needs a paid plan"}`)
			return
		}
		gets.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ip": strings.TrimPrefix(r.URL.Path, "/"), "is_vpn": false, "trial_expired": true,
		})
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "free-key")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache := gConfig, fBaseURL, fNoCache
	t.Cleanup(func() { gConfig, fBaseURL, fNoCache = savedConfig, savedBaseURL, savedNoCache })
	gConfig, fBaseURL, fNoCache = NewConfig(), api.URL, true

	addrs := []string{"9.1.0.1", "9.1.0.2", "9.1.0.3"}
	out, err := captureStdout(t, func() error {
		return runLookup(t.Context(), addrs, lookupOpts{format: lib.FormatJSONL})
	})
	if err != nil {
		t.Fatal(err)
	}
	if b, g := batches.Load(), gets.Load(); b != 1 || int(g) != len(addrs) {
		t.Errorf("sent %d batch(es) and %d GET(s), want 1 refused batch, then one GET for each of %d", b, g, len(addrs))
	}
	lines := 0
	for scanner := bufio.NewScanner(out); scanner.Scan(); lines++ {
		if strings.Contains(scanner.Text(), `"error"`) {
			t.Errorf("unanswered: %s", scanner.Text())
		}
	}
	if lines != len(addrs) {
		t.Errorf("wrote %d answer(s), want %d", lines, len(addrs))
	}
}

// Past its address's day a lookup without a key is answered 429 with no Retry-After, which the SDK
// never retries, and every address after it would be refused the same way, so none is sent: each
// carries the API's message, which names where to get a free key.
func TestASpentKeylessDaySendsNoMoreLookups(t *testing.T) {
	const spent = "lookups without an API key are used up for today; a free key answers on: " +
		"https://app.vpndetection.io/auth/signup"
	var gets atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = fmt.Fprintf(w, `{"error": %q}`, spent)
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache, savedWidth := gConfig, fBaseURL, fNoCache, fConcurrency
	t.Cleanup(func() {
		gConfig, fBaseURL, fNoCache, fConcurrency = savedConfig, savedBaseURL, savedNoCache, savedWidth
	})
	gConfig, fBaseURL, fNoCache, fConcurrency = NewConfig(), api.URL, true, 1

	addrs := []string{"9.2.0.1", "9.2.0.2", "9.2.0.3"}
	out, err := captureStdout(t, func() error {
		return runLookup(t.Context(), addrs, lookupOpts{format: lib.FormatJSONL})
	})
	if err != errUnanswered {
		t.Errorf("runLookup = %v, want errUnanswered", err)
	}
	if g := gets.Load(); g != 1 {
		t.Errorf("sent %d lookup(s), want 1: none after the day was spent, and that one never retried", g)
	}
	lines := 0
	for scanner := bufio.NewScanner(out); scanner.Scan(); lines++ {
		if !strings.Contains(scanner.Text(), "quota_exceeded (HTTP 429): "+spent) {
			t.Errorf("wrote %s, want the API's message", scanner.Text())
		}
	}
	if lines != len(addrs) {
		t.Errorf("wrote %d line(s), want one for each of %d", lines, len(addrs))
	}
}

// A lookup exits 1 when any address went unanswered, so a refused key or a
// spent allowance fails a script, and the output stays what it was: every
// answer, and each failure in place of its address's answer.
func TestLookupReportsAnUnansweredAddress(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"results": {"1.1.1.1": {"ip": "1.1.1.1", "is_vpn": false}},`+
			` "errors": {"2.2.2.2": {"status": 429, "error": "quota exceeded"}}}`)
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "test-key")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache := gConfig, fBaseURL, fNoCache
	t.Cleanup(func() { gConfig, fBaseURL, fNoCache = savedConfig, savedBaseURL, savedNoCache })
	gConfig, fBaseURL, fNoCache = NewConfig(), api.URL, true

	out, err := captureStdout(t, func() error {
		return runLookup(t.Context(), []string{"1.1.1.1", "2.2.2.2"}, lookupOpts{format: lib.FormatJSONL})
	})
	if err != errUnanswered {
		t.Errorf("runLookup = %v, want errUnanswered", err)
	}
	written, err := io.ReadAll(out)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ip":"1.1.1.1","is_vpn":false,"is_bogon":false}` + "\n" +
		`{"error":"vpndetection: quota_exceeded (HTTP 429): quota exceeded","ip":"2.2.2.2"}` + "\n"
	if string(written) != want {
		t.Errorf("wrote\n%s\nwant\n%s", written, want)
	}
}

// With --csv a refused lookup's error goes to standard error, since the row has no column for it,
// and the row holds its address alone.
func TestCSVReportsAnUnansweredAddressOnStandardError(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error": "invalid API key"}`)
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "test-key")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache := gConfig, fBaseURL, fNoCache
	t.Cleanup(func() { gConfig, fBaseURL, fNoCache = savedConfig, savedBaseURL, savedNoCache })
	gConfig, fBaseURL, fNoCache = NewConfig(), api.URL, true

	var out *os.File
	errs, err := captureFile(t, &os.Stderr, func() error {
		var lookupErr error
		out, lookupErr = captureStdout(t, func() error {
			return runLookup(t.Context(), []string{"1.1.1.1", "2.2.2.2"},
				lookupOpts{format: lib.FormatCSV, fields: []string{"ip", "is_vpn"}})
		})
		return lookupErr
	})
	if err != errUnanswered {
		t.Errorf("runLookup = %v, want errUnanswered", err)
	}
	written, readErr := io.ReadAll(out)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if want := "ip,is_vpn\n1.1.1.1,\n2.2.2.2,\n"; string(written) != want {
		t.Errorf("wrote\n%s\nwant\n%s", written, want)
	}
	reported, readErr := io.ReadAll(errs)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		want := "err: " + ip + ": vpndetection: unauthorized (HTTP 401)"
		if !strings.Contains(string(reported), want) {
			t.Errorf("standard error lacks %q:\n%s", want, reported)
		}
	}
}

// A list file that starts with a byte order mark, as Excel's "CSV UTF-8" export
// writes one, loses none of its addresses: the first is not the mark glued to it.
func TestLookupReadsAListAfterAByteOrderMark(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	t.Setenv("VPNDETECTION_API_KEY", "test-key")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache := gConfig, fBaseURL, fNoCache
	t.Cleanup(func() { gConfig, fBaseURL, fNoCache = savedConfig, savedBaseURL, savedNoCache })
	gConfig, fBaseURL, fNoCache = NewConfig(), api.URL, true

	file := filepath.Join(t.TempDir(), "ips.csv")
	if err := os.WriteFile(file, []byte("\xef\xbb\xbf1.1.1.1\r\n8.8.8.8\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error {
		return runLookup(t.Context(), []string{file},
			lookupOpts{format: lib.FormatCSV, fields: []string{"ip"}, input: iputil.Opts{Files: true}})
	})
	if err != nil {
		t.Fatal(err)
	}
	written, err := io.ReadAll(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := "ip\n1.1.1.1\n8.8.8.8\n"; string(written) != want {
		t.Errorf("wrote\n%s\nwant\n%s", written, want)
	}
}

// A core answer leaves the AS's type out, which sdk-go reads as "" until its
// next major: every lookup prints it absent, the one served from the cache too,
// while a paid answer's "" for an address no AS announces stays "", and a bogon's,
// which the SDK answers itself, keeps its own.
func TestAnASTypeTheAnswerLeftOutIsAbsentNotEmpty(t *testing.T) {
	var requests atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		ip := strings.TrimPrefix(r.URL.Path, "/")
		asn := map[string]any{"asn": 9009, "name": "M247 Europe SRL", "domain": "m247global.com",
			"prefix": "146.70.116.0/24", "country": "RO", "rpki_status": "not_found"}
		if ip == "9.1.0.2" {
			asn["type"] = ""
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ip": ip, "is_vpn": true, "asn": asn})
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL, savedNoCache := gConfig, fBaseURL, fNoCache
	t.Cleanup(func() { gConfig, fBaseURL, fNoCache = savedConfig, savedBaseURL, savedNoCache })
	gConfig, fBaseURL, fNoCache = NewConfig(), api.URL, false

	for _, run := range []string{"fetched", "cached"} {
		out, err := captureStdout(t, func() error {
			addrs := []string{"9.1.0.1", "9.1.0.2", "10.0.0.1"}
			return runLookup(t.Context(), addrs, lookupOpts{format: lib.FormatJSONL})
		})
		if err != nil {
			t.Fatal(err)
		}
		written, err := io.ReadAll(out)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(written)), "\n")
		if len(lines) != 3 {
			t.Fatalf("%s: wrote %d line(s), want 3:\n%s", run, len(lines), written)
		}
		for _, line := range lines {
			served := !strings.Contains(line, `"9.1.0.1"`)
			if got := strings.Contains(line, `"type":""`); got != served {
				t.Errorf("%s: asn.type printed is %v, want %v:\n%s", run, got, served, line)
			}
		}
	}
	if n := requests.Load(); n != 2 {
		t.Errorf("sent %d request(s), want 2: the second run reads the cache", n)
	}
}

// captureStdout runs fn with os.Stdout pointed at a file, and returns that file
// rewound for reading, with fn's error.
func captureStdout(t *testing.T, fn func() error) (*os.File, error) {
	t.Helper()
	return captureFile(t, &os.Stdout, fn)
}

// captureFile runs fn with *std, os.Stdout or os.Stderr, pointed at a file, and returns that file
// rewound for reading, with fn's error.
func captureFile(t *testing.T, std **os.File, fn func() error) (*os.File, error) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "std")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })

	saved := *std
	*std = file
	fnErr := fn()
	*std = saved
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	return file, fnErr
}

// A typed argument is an address, a range, a CIDR or a file that exists: anything else is named
// back rather than skipped, as a list file's unparseable lines are.
func TestCheckArgs(t *testing.T) {
	file := filepath.Join(t.TempDir(), "addresses.txt")
	if err := os.WriteFile(file, []byte("ip\n8.8.8.8\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		arg  string
		want string
	}{
		{"8.8.8.8", ""},
		{"2001:db8::1", ""},
		{"10.0.0.0/30", ""},
		{"1.1.1.1-1.1.1.3", ""},
		{file, ""},
		{"adresses.txt", "not an IP, CIDR, range or readable file"},
		{"not-an-ip", "not an IP, CIDR, range or readable file"},
		{"AS15169", "is an AS number"},
	}
	for _, c := range cases {
		err := checkArgs([]string{"8.8.8.8", c.arg})
		if c.want == "" && err != nil {
			t.Errorf("%q: %v, want no error", c.arg, err)
		}
		if c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%q: %v, want an error containing %q", c.arg, err, c.want)
		}
	}
}
