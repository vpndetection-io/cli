package lib

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
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
// goes rather than marshalled at the end.
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

func TestPrettyShowAbsent(t *testing.T) {
	got := render(t, FormatPretty, Opts{ShowAbsent: true}, freeAnswer)
	if !strings.Contains(got, "is_hosting") {
		t.Errorf("--show-absent should render it:\n%s", got)
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
