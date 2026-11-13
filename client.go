package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	vpndetection "github.com/vpndetection-io/sdk-go/v5"
	"golang.org/x/sync/errgroup"
)

// Client is the SDK client plus this CLI's own disk cache.
//
// The SDK has no cache interface to plug into, so the read-through lives here
// and the SDK's in-memory cache is switched OFF: it would be a second copy of
// every answer for the length of one short-lived process, and on a `bulk` over
// a large range that copy is the thing that decides whether the command fits in
// memory.
type Client struct {
	api   *vpndetection.Client
	cache *Cache
	key   string
}

// NewClient builds the client the current invocation should use.
func NewClient() (*Client, error) {
	key, _ := gConfig.ResolveKey()

	opts := []vpndetection.Option{
		vpndetection.WithHTTPClient(namedHTTPClient()),
		vpndetection.WithoutCache(),
		vpndetection.WithConcurrency(resolveConcurrency()),
		vpndetection.WithRetries(resolveRetries()),
	}
	if key != "" {
		opts = append(opts, vpndetection.WithAPIKey(key))
	}
	baseURL := gConfig.ResolveBaseURL()
	if baseURL != "" {
		opts = append(opts, vpndetection.WithBaseURL(baseURL))
	}

	api, err := vpndetection.New(opts...)
	if err != nil {
		return nil, err
	}
	touchSession(key)

	c := &Client{api: api, key: key}
	if gConfig.CacheEnabled && !fNoCache {
		cache, err := OpenCache(key, baseURL, gConfig.cacheTTL())
		if err != nil {
			// Not fatal: a cache is an optimization, and a locked database in
			// another terminal should slow this run down rather than stop it.
			fmt.Fprintf(os.Stderr, "warn: cache unavailable, continuing without it: %v\n", err)
		} else {
			c.cache = cache
		}
	}
	return c, nil
}

// namedHTTPClient is the SDK's own default, a 30 second timeout, on a transport
// that names this CLI on every request ahead of the SDK.
func namedHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second, Transport: agentTransport{next: http.DefaultTransport}}
}

// agentTransport puts vpndetection-cli/<version> first in the User-Agent the
// SDK has already set, vpndetection-go/<version>.
type agentTransport struct {
	next http.RoundTripper
}

func (t agentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// A RoundTripper must not modify the request it is given.
	named := req.Clone(req.Context())
	agent := "vpndetection-cli/" + version
	if sdk := req.Header.Get("User-Agent"); sdk != "" {
		agent += " " + sdk
	}
	named.Header.Set("User-Agent", agent)
	return t.next.RoundTrip(named)
}

// Close releases the cache.
func (c *Client) Close() {
	_ = c.cache.Close()
}

// HasKey reports whether this invocation is authenticated.
func (c *Client) HasKey() bool { return c.key != "" }

// LookupBatch classifies many addresses, serving what it can from cache and
// asking only for the rest.
//
// The SDK's batch keeps its own semantics - answers keyed by address,
// duplicates collapsed, a failing address carrying its error rather than
// failing the batch - so nothing here re-implements concurrency. POST /batch
// needs a paid plan, so without a key each address is its own lookup
// (lookupEach), and so it is for a key on the free plan, Unlimited, whose
// batch of more than one address the API answers 402.
//
// A bogon is answered by the SDK without a request and is not cached: it costs
// nothing to recompute and would otherwise take a slot.
func (c *Client) LookupBatch(
	ctx context.Context, ips []string,
) (map[string]vpndetection.BatchResult, error) {
	out := make(map[string]vpndetection.BatchResult, len(ips))
	miss := make([]string, 0, len(ips))
	for _, ip := range ips {
		if _, done := out[ip]; done {
			continue
		}
		if c.api.IsBogon(ip) {
			miss = append(miss, ip)
			continue
		}
		if hit := c.cache.Get(ip); hit != nil {
			out[ip] = vpndetection.BatchResult{Result: hit}
			continue
		}
		miss = append(miss, ip)
	}

	if len(miss) == 0 {
		return out, nil
	}

	var answers map[string]vpndetection.BatchResult
	if c.HasKey() {
		var err error
		if answers, err = c.api.LookupBatch(ctx, miss); err != nil {
			return out, err
		}
		if len(miss) > 1 && batchNeedsPaid(answers) {
			answers = c.lookupEach(ctx, miss)
		}
	} else {
		answers = c.lookupEach(ctx, miss)
	}
	fresh := make(map[string]*vpndetection.Result, len(answers))
	for ip, answer := range answers {
		out[ip] = answer
		// Errors are never cached: a 429 or a dropped connection is a fact
		// about this moment, and caching it would make the next run report a
		// failure that is no longer happening.
		if answer.Err == nil && answer.Result != nil && !answer.Result.IsBogon {
			fresh[ip] = answer.Result
		}
	}
	c.cache.PutBatch(fresh)
	return out, nil
}

// lookupEach answers addresses one GET each, at the batch width, for a caller
// whose batch is refused: no key, or a key on the free plan. Past the API's
// rate for Unlimited, a lookup is answered 429 with Retry-After, which the SDK
// waits out and retries.
func (c *Client) lookupEach(ctx context.Context, ips []string) map[string]vpndetection.BatchResult {
	answers := make([]vpndetection.BatchResult, len(ips))
	group := new(errgroup.Group)
	group.SetLimit(resolveConcurrency())
	for i, ip := range ips {
		group.Go(func() error {
			r, err := c.api.Lookup(ctx, ip)
			answers[i] = vpndetection.BatchResult{Result: r, Err: err}
			return nil
		})
	}
	// Each lookup records its own outcome and returns nil, so this only waits.
	_ = group.Wait()
	out := make(map[string]vpndetection.BatchResult, len(ips))
	for i, ip := range ips {
		out[ip] = answers[i]
	}
	return out
}

// batchNeedsPaid reports whether the API refused a whole batch as a paid
// plan's: every address carrying its 402.
func batchNeedsPaid(answers map[string]vpndetection.BatchResult) bool {
	if len(answers) == 0 {
		return false
	}
	for _, a := range answers {
		var apiErr *vpndetection.Error
		if !errors.As(a.Err, &apiErr) || apiErr.StatusCode != http.StatusPaymentRequired {
			return false
		}
	}
	return true
}

// Database is the licensed-dataset half of the API.
func (c *Client) Database() *vpndetection.DatabaseAPI { return c.api.Database }

// requireKey refuses a command that cannot work unauthenticated, naming how to
// fix it rather than reporting a 401 from three layers down.
func (c *Client) requireKey(what string) error {
	if c.HasKey() {
		return nil
	}
	return fmt.Errorf(
		"%s needs an API key; run `%s login`, or pass --key, or set VPNDETECTION_API_KEY",
		what, progBase,
	)
}

// resolveConcurrency is the batch width: flag, then config, then the SDK's own
// default.
func resolveConcurrency() int {
	if fConcurrency > 0 {
		return fConcurrency
	}
	if gConfig.Concurrency > 0 {
		return gConfig.Concurrency
	}
	return defaultConcurrency
}

// resolveRetries is how many times a retryable failure is retried.
func resolveRetries() int {
	if fRetries >= 0 {
		return fRetries
	}
	if gConfig.Retries >= 0 {
		return gConfig.Retries
	}
	return defaultRetries
}

// explain turns an SDK error into something worth reading at a terminal.
//
// The SDK's own message is accurate and says nothing about what to DO, which
// for the two authentication failures is the entire question.
func explain(err error) error {
	var apiErr *vpndetection.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.Kind {
	case vpndetection.KindUnauthorized:
		return fmt.Errorf("%w\nthe API key was not accepted; check `%s whoami`", err, progBase)
	case vpndetection.KindForbidden:
		return fmt.Errorf("%w\nthis key's plan or license does not cover that", err)
	case vpndetection.KindQuotaExceeded:
		return fmt.Errorf("%w\nsee `%s whoami` for the allowance and when it resets", err, progBase)
	case vpndetection.KindRateLimited:
		return fmt.Errorf("%w\nslow down, or lower --concurrency", err)
	}
	return err
}
