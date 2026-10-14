package lib

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	vpndetection "github.com/vpndetection-io/sdk-go/v5"
	"gopkg.in/yaml.v3"
)

func render(t *testing.T, format Format, opts Opts, bodies ...string) string {
	t.Helper()
	var out strings.Builder
	w := NewWriter(&out, format, opts)
	for _, body := range bodies {
		var v any
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			t.Fatal(err)
		}
		ip, _ := v.(map[string]any)["ip"].(string)
		if err := w.Write(ip, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// CSV is the one format where absent and false have to be told apart by
// convention: an empty cell is "not in your plan", the literal false is
// "checked, and no".
func TestCSVAbsentIsEmptyAndFalseIsFalse(t *testing.T) {
	got := render(t, FormatCSV, Opts{}, freeAnswer)
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected a header and one row, got %d lines", len(lines))
	}
	header := strings.Split(lines[0], ",")
	row := strings.Split(lines[1], ",")
	cell := func(name string) string {
		for i, h := range header {
			if h == name && i < len(row) {
				return row[i]
			}
		}
		t.Fatalf("no column %q", name)
		return ""
	}
	if cell("is_vpn") != "true" {
		t.Errorf("is_vpn = %q", cell("is_vpn"))
	}
	// Served and false.
	if cell("is_bogon") != "false" {
		t.Errorf("is_bogon = %q, want false", cell("is_bogon"))
	}
	// Never served.
	if cell("is_hosting") != "" {
		t.Errorf("is_hosting = %q, want empty (absent)", cell("is_hosting"))
	}
}

// The header is the CSV contract and must not move with the answer.
func TestCSVHeaderIsFixed(t *testing.T) {
	free := strings.Split(render(t, FormatCSV, Opts{}, freeAnswer), "\n")[0]
	max := strings.Split(render(t, FormatCSV, Opts{}, maxAnswer), "\n")[0]
	if free != max {
		t.Errorf("the header changed with the plan:\n free: %s\n max:  %s", free, max)
	}
	if free != strings.Join(Columns, ",") {
		t.Errorf("header is not Columns: %s", free)
	}
}

// An empty run still emits the header, so a consumer never has to special-case
// a zero-row file.
func TestCSVEmptyStillHasHeader(t *testing.T) {
	got := render(t, FormatCSV, Opts{})
	if strings.TrimSpace(got) != strings.Join(Columns, ",") {
		t.Errorf("got %q", got)
	}
}

// The JSON writer streams, so the object has to be assembled correctly as it
// goes rather than marshaled at the end.
func TestJSONStreamsValidObject(t *testing.T) {
	got := render(t, FormatJSON, Opts{}, freeAnswer, maxAnswer)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, got)
	}
	if len(parsed) != 2 {
		t.Errorf("got %d answers, want 2", len(parsed))
	}
	if _, ok := parsed["1.1.1.1"]; !ok {
		t.Errorf("not keyed by address: %v", keys(parsed))
	}
}

func TestJSONEmptyIsAnEmptyObject(t *testing.T) {
	got := strings.TrimSpace(render(t, FormatJSON, Opts{}))
	if got != "{}" {
		t.Errorf("got %q, want {}", got)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Errorf("not valid JSON: %v", err)
	}
}

// A failing address appears in the output beside the good ones rather than
// vanishing, or a bulk run silently answers fewer addresses than it was given.
func TestJSONCarriesPerAddressErrors(t *testing.T) {
	var out strings.Builder
	w := NewWriter(&out, FormatJSON, Opts{})
	if err := w.WriteError("9.9.9.9", errors.New("rate limit exceeded")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]map[string]string
	if err := json.Unmarshal([]byte(out.String()), &parsed); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out.String())
	}
	if parsed["9.9.9.9"]["error"] != "rate limit exceeded" {
		t.Errorf("got %v", parsed)
	}
}

func TestJSONLIsOnePerLine(t *testing.T) {
	got := strings.TrimSpace(render(t, FormatJSONL, Opts{}, freeAnswer, maxAnswer))
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), got)
	}
	for _, line := range lines {
		var v map[string]any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Errorf("line is not valid JSON: %v", err)
		}
	}
}

// YAML says what JSON says: the API's keys, nothing nested under the SDK's type
// name, a field the plan leaves out left out rather than null, and an empty
// detail object as {}. The answer arrives as the SDK's Result here, as it does
// from a lookup, because a body decoded into maps cannot show the difference.
func TestYAMLSaysWhatJSONSays(t *testing.T) {
	selections := []Opts{{}, {Fields: []string{"ip", "is_tor", "vpn.provider", "relay.provider", "hosting"}}}
	for _, body := range []string{freeAnswer, maxAnswer} {
		var answer vpndetection.Result
		if err := json.Unmarshal([]byte(body), &answer); err != nil {
			t.Fatal(err)
		}
		for _, opts := range selections {
			var fromJSON, fromYAML map[string]any
			asJSON := renderAnswer(t, FormatJSON, opts, &answer)
			if err := json.Unmarshal([]byte(asJSON), &fromJSON); err != nil {
				t.Fatal(err)
			}
			asYAML := renderAnswer(t, FormatYAML, opts, &answer)
			if err := yaml.Unmarshal([]byte(asYAML), &fromYAML); err != nil {
				t.Fatal(err)
			}
			wantJSON, _ := json.Marshal(fromJSON)
			gotJSON, _ := json.Marshal(fromYAML)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("-f %v: YAML differs from JSON:\n%s\nreads as %s\nwant %s",
					opts.Fields, asYAML, gotJSON, wantJSON)
			}
		}
	}
}

func renderAnswer(t *testing.T, format Format, opts Opts, answer *vpndetection.Result) string {
	t.Helper()
	var out strings.Builder
	w := NewWriter(&out, format, opts)
	if err := w.Write(answer.IP, answer); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// --field narrows every format, not CSV alone: each keeps the fields named and
// nothing else, and the formats that print an object keep the answer's nesting.
func TestFieldSelectsInEveryFormat(t *testing.T) {
	opts := Opts{Fields: []string{"ip", "is_vpn", "vpn.provider", "hosting"}}
	cases := []struct {
		format Format
		want   string
	}{
		{FormatJSON, `{
  "146.70.22.220": {
    "hosting": {
      "confidence": "high",
      "last_seen": "2026-09-11",
      "provider": "m247"
    },
    "ip": "146.70.22.220",
    "is_vpn": true,
    "vpn": {
      "provider": "mullvad"
    }
  }
}
`},
		{FormatJSONL, `{"hosting":{"confidence":"high","last_seen":"2026-09-11","provider":"m247"},` +
			`"ip":"146.70.22.220","is_vpn":true,"vpn":{"provider":"mullvad"}}` + "\n"},
		{FormatYAML, `146.70.22.220:
  hosting:
    confidence: high
    last_seen: "2026-09-11"
    provider: m247
  ip: 146.70.22.220
  is_vpn: true
  vpn:
    provider: mullvad
`},
		{FormatCSV, "ip,is_vpn,vpn.provider,hosting.provider,hosting.confidence,hosting.last_seen\n" +
			"146.70.22.220,true,mullvad,m247,high,2026-09-11\n"},
		{FormatPretty, `ip                  146.70.22.220
is_vpn              true
vpn.provider        mullvad
hosting.provider    m247
hosting.confidence  high
hosting.last_seen   2026-09-11
`},
	}
	for _, c := range cases {
		if got := render(t, c.format, opts, maxAnswer); got != c.want {
			t.Errorf("%s:\n%s\nwant\n%s", c.format, got, c.want)
		}
	}
}

// A selected field the answer does not carry stays out rather than printing as
// null, and a detail object that arrived empty stays {} whichever of its fields
// was named, so absent and false stay apart in a selection too.
func TestFieldKeepsAbsentAndEmptyApart(t *testing.T) {
	opts := Opts{Fields: []string{"is_hosting", "relay.provider", "tor"}}
	if got := render(t, FormatJSONL, opts, freeAnswer); got != "{}\n" {
		t.Errorf("free plan: got %q, want {} since it carries none of them", got)
	}
	want := `{"is_hosting":true,"relay":{},"tor":{}}` + "\n"
	if got := render(t, FormatJSONL, opts, maxAnswer); got != want {
		t.Errorf("max plan: got %q, want %q", got, want)
	}
}

// An error is not an answer, so a selection leaves it whole: it still names its
// address and says what went wrong.
func TestFieldLeavesErrorsWhole(t *testing.T) {
	cases := map[Format]string{
		FormatJSON:  "{\n  \"9.9.9.9\": {\n    \"error\": \"rate limit exceeded\"\n  }\n}\n",
		FormatJSONL: `{"error":"rate limit exceeded","ip":"9.9.9.9"}` + "\n",
		FormatYAML:  "9.9.9.9:\n  error: rate limit exceeded\n",
	}
	for format, want := range cases {
		var out strings.Builder
		w := NewWriter(&out, format, Opts{Fields: []string{"is_vpn"}})
		if err := w.WriteError("9.9.9.9", errors.New("rate limit exceeded")); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if out.String() != want {
			t.Errorf("%s: got %q, want %q", format, out.String(), want)
		}
	}
}

// Pretty output prints ONLY what was served, and says nothing about the rest.
// It used to close with a summary of the absent fields, which on the free tier
// was longer than the answer and appeared under every lookup.
func TestPrettyOmitsAbsentAndSaysNothingAboutIt(t *testing.T) {
	got := render(t, FormatPretty, Opts{}, freeAnswer)
	if strings.Contains(got, "is_hosting") {
		t.Errorf("an absent field was printed:\n%s", got)
	}
	if strings.Contains(got, "not included") || strings.Contains(got, "plan") {
		t.Errorf("output should carry no note about the plan:\n%s", got)
	}
	if !strings.Contains(got, "is_vpn") {
		t.Errorf("a served field is missing:\n%s", got)
	}
}

// An empty string prints as "", never as the {} that says a flag is false: the
// tor dataset carries no provider, and an unattributed VPN range has none.
func TestPrettyTellsAnEmptyStringFromAnEmptyObject(t *testing.T) {
	got := render(t, FormatPretty, Opts{}, `{"ip":"1.1.1.1","is_vpn":false,"is_tor":true,`+
		`"vpn":{},"tor":{"provider":"","confidence":"high","last_seen":"2026-09-12"}}`)
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
		name, value, _ := strings.Cut(line, " ")
		values[name] = strings.TrimSpace(value)
	}
	if values["tor.provider"] != `""` {
		t.Errorf("tor.provider = %q, want \"\"\n%s", values["tor.provider"], got)
	}
	if values["vpn"] != "{}" {
		t.Errorf("vpn = %q, want {}\n%s", values["vpn"], got)
	}
}

func TestPrettyShowAbsent(t *testing.T) {
	got := render(t, FormatPretty, Opts{ShowAbsent: true}, freeAnswer)
	if !strings.Contains(got, "is_hosting") {
		t.Errorf("--show-absent should render it:\n%s", got)
	}
}

// --show-absent prints a detail object that arrived empty as {}, as the block
// does by default, never as a '-' for each of its fields, which says the plan
// leaves them out. An object the plan does leave out still prints that way.
func TestPrettyShowAbsentPrintsAnEmptyObjectAsItself(t *testing.T) {
	rows := func(body string) map[string]string {
		out := map[string]string{}
		got := render(t, FormatPretty, Opts{ShowAbsent: true}, body)
		for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
			name, value, _ := strings.Cut(line, " ")
			out[name] = strings.TrimSpace(value)
		}
		return out
	}
	max := rows(maxAnswer)
	if _, split := max["relay.provider"]; max["relay"] != "{}" || split {
		t.Errorf("max: relay = %q, want {} and no relay.provider row: %v", max["relay"], max)
	}
	free := rows(freeAnswer)
	if _, whole := free["relay"]; free["relay.provider"] != "-" || whole {
		t.Errorf("free: relay.provider = %q, want - and no relay row: %v", free["relay.provider"], free)
	}
}

// So does a selection naming the object or any of its fields. Its fields stay
// '-' where the plan leaves the object out.
func TestFieldPrintsAnEmptyObjectAsItself(t *testing.T) {
	cases := []struct {
		body   string
		fields []string
		want   string
	}{
		{maxAnswer, []string{"relay"}, "relay  {}\n"},
		{maxAnswer, []string{"is_relay", "relay.provider", "relay.last_seen"}, "is_relay  false\nrelay     {}\n"},
		{freeAnswer, []string{"relay"}, "relay.provider    -\nrelay.confidence  -\nrelay.last_seen   -\n"},
	}
	for _, c := range cases {
		if got := render(t, FormatPretty, Opts{Fields: c.fields}, c.body); got != c.want {
			t.Errorf("-f %v:\n%s\nwant\n%s", c.fields, got, c.want)
		}
	}
}

// The readable block lists each detail object's fields together, the ones
// appended to Columns after the rest included, by default and with
// --show-absent alike.
func TestPrettyKeepsEachObjectTogether(t *testing.T) {
	for name, got := range map[string]string{
		"default":       render(t, FormatPretty, Opts{}, proxyAnswer),
		"--show-absent": render(t, FormatPretty, Opts{ShowAbsent: true}, freeAnswer),
	} {
		done := map[string]bool{}
		last := ""
		for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
			field, _, _ := strings.Cut(line, " ")
			obj, _, _ := strings.Cut(field, ".")
			if obj == last {
				continue
			}
			if done[obj] {
				t.Errorf("%s splits %s's fields:\n%s", name, obj, got)
				break
			}
			done[last] = true
			last = obj
		}
	}
}

// --field takes the proxy fields Columns gained last, alone or with the rest of
// their object, and CSV prints them.
func TestFieldTakesEveryProxyField(t *testing.T) {
	cases := []struct {
		fields []string
		want   string
	}{
		{[]string{"ip", "resproxy.hits_days_pct", "resproxy.providers_num"},
			"ip,resproxy.hits_days_pct,resproxy.providers_num\n217.80.249.248,19,18\n"},
		{[]string{"mobproxy"}, "mobproxy.provider,mobproxy.first_seen,mobproxy.last_seen,mobproxy.hits," +
			"mobproxy.hits_days_pct,mobproxy.providers_num\np,2026-09-30,2026-10-11,3,2,1\n"},
	}
	for _, c := range cases {
		if err := ValidateFields(c.fields); err != nil {
			t.Errorf("-f %v: %v", c.fields, err)
			continue
		}
		if got := render(t, FormatCSV, Opts{Fields: c.fields}, proxyAnswer); got != c.want {
			t.Errorf("-f %v:\n%s\nwant\n%s", c.fields, got, c.want)
		}
	}
}

func TestParseFormat(t *testing.T) {
	for in, want := range map[string]Format{
		"": FormatPretty, "pretty": FormatPretty, "JSON": FormatJSON,
		"jsonl": FormatJSONL, "ndjson": FormatJSONL, "csv": FormatCSV,
		"yaml": FormatYAML, "yml": FormatYAML,
	} {
		got, err := ParseFormat(in)
		if err != nil || got != want {
			t.Errorf("ParseFormat(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseFormat("xml"); err == nil {
		t.Error("expected an unknown format to be rejected")
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
