package lib

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	vpndetection "github.com/vpndetection-io/sdk-go/v5"
)

// freeAnswer is what a lookup without a key serves: two fields, and every other
// one ABSENT rather than false.
const freeAnswer = `{"ip":"1.1.1.1","is_vpn":true,"is_bogon":false}`

// maxAnswer is the full shape Max serves, with the method it served until
// 2026-10-20, and a detail object that is present but EMPTY - which means the
// flag above it is false, not that it is missing.
const maxAnswer = `{
  "ip":"146.70.22.220","is_vpn":true,"is_hosting":true,"is_relay":false,
  "is_tor":false,"is_cdn":false,"is_bot":false,"is_resproxy":false,"is_dcproxy":false,
  "is_mobproxy":false,
  "vpn":{"provider":"mullvad","provider_name":"Mullvad","last_seen":"2026-09-12","confidence":"high",
    "method":"scan"},
  "hosting":{"provider":"m247","provider_name":"M247","confidence":"high","last_seen":"2026-09-11"},
  "relay":{},"tor":{},"cdn":{},"bot":{},"resproxy":{},"dcproxy":{},"mobproxy":{},
  "is_bogon":false
}`

// proxyAnswer carries three proxy families' detail objects populated, as the API
// serves a populated one: with every key, the ones Columns gained last included.
const proxyAnswer = `{
  "ip":"217.80.249.248","is_resproxy":true,"is_dcproxy":false,"is_mobproxy":true,
  "is_pubproxy":true,
  "resproxy":{"provider":"","provider_name":"","first_seen":"2026-07-16","last_seen":"2026-10-12","hits":120,
    "hits_days_pct":19,"providers_num":18},
  "dcproxy":{},
  "mobproxy":{"provider":"p","provider_name":"P","first_seen":"2026-09-30","last_seen":"2026-10-11","hits":3,
    "hits_days_pct":2,"providers_num":1},
  "pubproxy":{"entry":false,"exit":true,"protocols":["socks5"],"num_ports":0,"num_proxies":1,
    "last_seen":"2026-10-17","last_usable":"2026-10-17"},
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

// An empty detail object is PRESENT and means its flag is false, which is not
// the answer a plan without that object gives.
func TestEmptyObjectIsPresentNotAbsent(t *testing.T) {
	max := flattenJSON(t, maxAnswer)
	if c := max.Get("relay"); !c.Present || c.Text != "" {
		t.Errorf("relay should be present and empty, got %+v", c)
	}
	// The free answer has no relay object at all.
	free := flattenJSON(t, freeAnswer)
	if c := free.Get("relay"); c.Present {
		t.Errorf("free: relay should be absent, got %+v", c)
	}
}

// Present() orders by meaning, not alphabetically: the address, then the flags,
// then each detail object with its own fields.
func TestPresentOrder(t *testing.T) {
	got := flattenJSON(t, maxAnswer).Present()
	want := []string{
		"ip", "is_bogon",
		"is_vpn", "is_hosting", "is_relay", "is_tor",
		"is_cdn", "is_bot", "is_resproxy", "is_dcproxy", "is_mobproxy",
		"vpn.provider", "vpn.provider_name", "vpn.last_seen", "vpn.confidence", "vpn.method",
		"hosting.provider", "hosting.provider_name", "hosting.confidence", "hosting.last_seen",
		"relay", "tor", "cdn", "bot", "resproxy", "dcproxy", "mobproxy",
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

// freeKeyAnswer is what Unlimited serves since 2026-11-20: Tor, relay and anycast
// beside the VPN flag, the location and AS, and the hostname, empty where the
// address has none.
const freeKeyAnswer = `{
  "ip":"146.70.116.130","is_vpn":true,"is_relay":false,"is_tor":false,"is_anycast":false,
  "relay":{},"tor":{},
  "location":{"continent":"EU","continent_name":"Europe","country":"AT","country_name":"Austria",
    "timezone":"Europe/Vienna","currency_code":"EUR","currency_name":"Euro","currency_symbol":"€",
    "flag_emoji":"🇦🇹","flag_unicode":"U+1F1E6 U+1F1F9"},
  "asn":{"asn":9009,"name":"M247 Europe SRL","domain":"m247global.com","type":"hosting",
    "prefix":"146.70.116.0/24","country":"RO","rpki_status":"not_found"},
  "hostname":"",
  "is_bogon":false
}`

// The location and the AS print after the detail objects, each with its own
// fields in Columns' order, its members from 1.15.0 among them.
func TestPresentListsLocationAndAsnAfterTheDetails(t *testing.T) {
	rec := flattenJSON(t, freeKeyAnswer)
	got := rec.Present()
	want := []string{"ip", "is_bogon", "is_vpn", "is_relay", "is_tor", "is_anycast", "relay", "tor"}
	for _, obj := range []string{"location.", "asn."} {
		for _, col := range Columns {
			if strings.HasPrefix(col, obj) && rec[col].Present {
				want = append(want, col)
			}
		}
	}
	want = append(want, "hostname")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

// --show-absent lists every field in its place, but never vpn.method, which keeps
// its CSV column though no answer carries it since 2026-10-20, and user_agent only
// for a lookup that sent an agent, since no plan answers it to one that did not.
func TestAllLeavesOutTheRetiredColumn(t *testing.T) {
	agentless := 0
	for _, col := range Columns {
		if !strings.HasPrefix(col, "user_agent.") {
			agentless++
		}
	}
	for _, c := range []struct {
		agentSent bool
		want      int
	}{{false, agentless - 1}, {true, len(Columns) - 1}} {
		got := flattenJSON(t, freeAnswer).All(c.agentSent)
		for _, f := range got {
			if f == "vpn.method" {
				t.Fatalf("All lists vpn.method, which no answer carries: %v", got)
			}
			if !c.agentSent && strings.HasPrefix(f, "user_agent.") {
				t.Fatalf("All lists %s for a lookup that sent no agent: %v", f, got)
			}
		}
		if len(got) != c.want {
			t.Errorf("agent sent %v: All lists %d fields, want %d: %v", c.agentSent, len(got), c.want, got)
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
	// A list prints as its JSON, so an empty one is not an absent field's blank.
	rec = flattenJSON(t, `{"pubproxy":{"protocols":["socks5","http"]}}`)
	if c := rec.Get("pubproxy.protocols"); c.Text != `["socks5","http"]` {
		t.Errorf("list: %q", c.Text)
	}
	rec = flattenJSON(t, `{"pubproxy":{"protocols":[]}}`)
	if c := rec.Get("pubproxy.protocols"); !c.Present || c.Text != "[]" {
		t.Errorf("empty list: %+v", c)
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
	want := []string{
		"is_vpn", "vpn.provider", "vpn.provider_name", "vpn.last_seen", "vpn.confidence", "vpn.method",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Columns names every field the SDK's answer can carry, and nothing else, so
// neither CSV nor --field leaves one out. A field a new sdk-go brings fails
// here, on the bump that brings it.
func TestColumnsAreTheSDKsFields(t *testing.T) {
	var answer vpndetection.Result
	fill(reflect.ValueOf(&answer).Elem())
	rec, err := Flatten(&answer)
	if err != nil {
		t.Fatal(err)
	}
	named := make(map[string]bool, len(Columns))
	for _, col := range Columns {
		named[col] = true
		if !rec[col].Present {
			t.Errorf("Columns names %s, which the SDK's answer does not carry", col)
		}
	}
	for k := range rec {
		if !named[k] {
			t.Errorf("the SDK's answer carries %s, which Columns does not name: "+
				"append it, so no CSV column moves", k)
		}
	}
}

// Flags and Objects are what the readable block lists Columns by, so a flag or
// an object Columns gains and they do not name prints apart from its family.
func TestFlagsAndObjectsFollowColumns(t *testing.T) {
	var flags, objects []string
	seen := map[string]bool{}
	for _, col := range Columns {
		if strings.HasPrefix(col, "is_") && col != "is_bogon" {
			flags = append(flags, col)
		}
		if obj, _, nested := strings.Cut(col, "."); nested && !seen[obj] {
			objects = append(objects, obj)
			seen[obj] = true
		}
	}
	if !reflect.DeepEqual(Flags, flags) {
		t.Errorf("Flags = %v, want the flags Columns names, in its order: %v", Flags, flags)
	}
	// Each flag's detail object in Flags' order, then the objects no flag owns,
	// in Columns' order, since a field joins Columns at its end.
	var want []string
	owned := map[string]bool{}
	for _, f := range Flags {
		if detail := strings.TrimPrefix(f, "is_"); seen[detail] {
			want = append(want, detail)
			owned[detail] = true
		}
	}
	for _, obj := range objects {
		if !owned[obj] {
			want = append(want, obj)
		}
	}
	if !reflect.DeepEqual(Objects, want) {
		t.Errorf("Objects = %v, want each flag's detail in Flags' order, then the rest: %v", Objects, want)
	}
	// Web IP's detail is a list, one column, and Anycast IP has none.
	for _, f := range Flags {
		detail := strings.TrimPrefix(f, "is_")
		if !seen[detail] && !slices.Contains(Columns, detail) && f != "is_anycast" {
			t.Errorf("%s has no detail in Columns", f)
		}
	}
}

// Each detail object's provider_name sits right after its provider in CSV's
// columns, where a reader looks for the name; TestPresentOrder pins the block.
func TestProviderNameFollowsItsProvider(t *testing.T) {
	names := 0
	for i, col := range Columns {
		obj, field, _ := strings.Cut(col, ".")
		if field == "provider" && (i+1 == len(Columns) || Columns[i+1] != obj+".provider_name") {
			t.Errorf("%s is not followed by %s.provider_name", col, obj)
		}
		if field == "provider_name" {
			names++
		}
	}
	if names != 10 {
		t.Errorf("Columns names %d provider_name fields, want 10", names)
	}
}

// fill gives every member of v a value, so that its JSON carries every key.
func fill(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fill(v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				fill(v.Field(i))
			}
		}
	}
}
