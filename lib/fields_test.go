package lib

import (
	"strings"
	"testing"
)

// freeAnswer is what the free plan serves: two fields, and every other one
// ABSENT rather than false.
const freeAnswer = `{"ip":"1.1.1.1","is_vpn":true,"is_bogon":false}`

// maxAnswer is the full shape, including a detail object that is present but
// EMPTY - which means the flag above it is false, not that it is missing.
const maxAnswer = `{
  "ip":"146.70.22.220","is_vpn":true,"is_hosting":true,"is_relay":false,
  "is_tor":false,"is_cdn":false,"is_resproxy":false,"is_dcproxy":false,
  "is_mobproxy":false,
  "vpn":{"provider":"mullvad","last_seen":"2026-09-12","confidence":"high","method":"scan"},
  "hosting":{"provider":"m247","confidence":"high","last_seen":"2026-09-11"},
  "relay":{},"tor":{},"cdn":{},"resproxy":{},"dcproxy":{},"mobproxy":{},
  "is_bogon":false
}`

func flattenJSON(t *testing.T, body string) Record {
	t.Helper()
	var v any
	if err := jsonUnmarshal(body, &v); err != nil {
		t.Fatal(err)
	}
	rec, err := Flatten(v)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// Absent and present-false are different answers and must stay different all
// the way to the output. This is the single subtlest thing about this product.
func TestAbsentIsNotFalse(t *testing.T) {
	free := flattenJSON(t, freeAnswer)
	max := flattenJSON(t, maxAnswer)

	// Free: is_hosting was never served.
	if c := free.Get("is_hosting"); c.Present {
		t.Error("free: is_hosting should be absent")
	}
	// Max: is_hosting was served, and is true.
	if c := max.Get("is_hosting"); !c.Present || c.Text != "true" {
		t.Errorf("max: is_hosting = %+v", c)
	}
	// Max: is_relay was served, and is false. Not the same as free's absence.
	if c := max.Get("is_relay"); !c.Present || c.Text != "false" {
		t.Errorf("max: is_relay = %+v", c)
	}
}

// An empty detail object is PRESENT and means its flag is false. Its keys are
// therefore not absent, or the summary reports a plan as narrower than it is.
func TestEmptyObjectIsPresentNotAbsent(t *testing.T) {
	max := flattenJSON(t, maxAnswer)

	if c := max.Get("relay"); !c.Present || c.Text != "" {
		t.Errorf("relay should be present and empty, got %+v", c)
	}
	for _, f := range max.Absent() {
		if strings.HasPrefix(f, "relay.") {
			t.Errorf("%s counted as absent, but relay arrived as an empty object", f)
		}
	}
	// The free answer has no relay object at all, so its keys ARE absent.
	free := flattenJSON(t, freeAnswer)
	found := false
	for _, f := range free.Absent() {
		if f == "relay.provider" {
			found = true
		}
	}
	if !found {
		t.Error("free: relay.provider should be absent")
	}
}

// Present() orders by meaning, not alphabetically: the address, then the flags,
// then each detail object with its own fields.
func TestPresentOrder(t *testing.T) {
	got := flattenJSON(t, maxAnswer).Present()
	want := []string{
		"ip", "is_bogon",
		"is_vpn", "is_hosting", "is_relay", "is_tor",
		"is_cdn", "is_resproxy", "is_dcproxy", "is_mobproxy",
		"vpn.provider", "vpn.last_seen", "vpn.confidence", "vpn.method",
		"hosting.provider", "hosting.confidence", "hosting.last_seen",
		"relay", "tor", "cdn", "resproxy", "dcproxy", "mobproxy",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d fields %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d: got %q, want %q\nfull: %v", i, got[i], want[i], got)
		}
	}
}

// A field the API grows after this build must still be printed, or the CLI
// silently hides what the customer is paying for.
func TestPresentKeepsUnknownFields(t *testing.T) {
	rec := flattenJSON(t, `{"ip":"1.1.1.1","is_vpn":false,"is_something_new":true}`)
	found := false
	for _, f := range rec.Present() {
		if f == "is_something_new" {
			found = true
		}
	}
	if !found {
		t.Errorf("an unrecognized field was dropped: %v", rec.Present())
	}
}

func TestScalarRendering(t *testing.T) {
	rec := flattenJSON(t, `{"ip":"1.1.1.1","is_vpn":false,"resproxy":{"provider":"p","hits":42}}`)
	if c := rec.Get("is_vpn"); c.Text != "false" {
		t.Errorf("bool: %q", c.Text)
	}
	// JSON has one number type; a hit count must not render as 42.000000.
	if c := rec.Get("resproxy.hits"); c.Text != "42" {
		t.Errorf("number: %q", c.Text)
	}
}

func TestValidateAndExpandFields(t *testing.T) {
	if err := ValidateFields([]string{"is_vpn", "vpn.provider", "vpn"}); err != nil {
		t.Errorf("valid fields rejected: %v", err)
	}
	// A typo is an error, not a column of blanks.
	err := ValidateFields([]string{"is_vpnn"})
	if err == nil || !strings.Contains(err.Error(), "is_vpnn") {
		t.Errorf("expected the typo to be named, got %v", err)
	}

	got := ExpandFields([]string{"is_vpn", "vpn"})
	want := []string{"is_vpn", "vpn.provider", "vpn.last_seen", "vpn.confidence", "vpn.method"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}
