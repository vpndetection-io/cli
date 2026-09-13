package lib

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

// Format is an output shape.
type Format string

const (
	// FormatPretty is the human-readable block, and the default for one answer.
	FormatPretty Format = "pretty"
	// FormatJSON is one object keyed by address, and the default for many.
	FormatJSON Format = "json"
	// FormatJSONL is one object per line, for pipelines.
	FormatJSONL Format = "jsonl"
	FormatCSV   Format = "csv"
	FormatYAML  Format = "yaml"
)

// ParseFormat resolves a --format value.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "pretty":
		return FormatPretty, nil
	case "json":
		return FormatJSON, nil
	case "jsonl", "ndjson":
		return FormatJSONL, nil
	case "csv":
		return FormatCSV, nil
	case "yaml", "yml":
		return FormatYAML, nil
	}
	return "", fmt.Errorf("unknown format %q; want pretty, json, jsonl, csv or yaml", s)
}

// Opts tunes how answers are rendered.
type Opts struct {
	// Fields selects specific dotted fields. Empty means all of them.
	Fields []string
	// ShowAbsent renders the fields this plan does not include, rather than
	// omitting them and noting the omission.
	ShowAbsent bool
	// Color enables ANSI colour in the pretty format.
	Color bool
}

// Writer streams answers in one format.
//
// Streaming rather than collecting, because bulk output is unbounded: a run
// over a /16 must start printing immediately and must never hold every answer
// to serialize them at the end.
type Writer interface {
	// Write emits one answer.
	Write(ip string, v any) error
	// WriteError emits one address's failure, so a bad entry appears in the
	// output next to the good ones instead of vanishing.
	WriteError(ip string, err error) error
	// Close finishes the stream: the closing brace, the final flush.
	Close() error
}

// NewWriter builds a writer for a format.
func NewWriter(w io.Writer, f Format, opts Opts) Writer {
	switch f {
	case FormatCSV:
		return &csvWriter{w: csv.NewWriter(w), opts: opts}
	case FormatJSONL:
		return &jsonlWriter{w: w}
	case FormatYAML:
		return &yamlWriter{w: w, answers: map[string]any{}}
	case FormatPretty:
		return &prettyWriter{w: w, opts: opts}
	default:
		return &jsonWriter{w: w}
	}
}

// jsonWriter emits one object keyed by address, written incrementally.
//
// The shape matches what the reference CLI produces, but the bytes go out as
// each answer arrives rather than from a map built in memory first.
type jsonWriter struct {
	w     io.Writer
	count int
	err   error
}

func (j *jsonWriter) Write(ip string, v any) error { return j.emit(ip, v) }

func (j *jsonWriter) WriteError(ip string, err error) error {
	return j.emit(ip, map[string]string{"error": err.Error()})
}

func (j *jsonWriter) emit(ip string, v any) error {
	if j.err != nil {
		return j.err
	}
	sep := ",\n"
	if j.count == 0 {
		sep = "{\n"
	}
	key, err := json.Marshal(ip)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(v, "  ", "  ")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(j.w, "%s  %s: %s", sep, key, body); err != nil {
		j.err = err
		return err
	}
	j.count++
	return nil
}

func (j *jsonWriter) Close() error {
	if j.err != nil {
		return j.err
	}
	if j.count == 0 {
		_, err := fmt.Fprintln(j.w, "{}")
		return err
	}
	_, err := fmt.Fprint(j.w, "\n}\n")
	return err
}

// jsonlWriter emits one compact object per line.
type jsonlWriter struct{ w io.Writer }

func (j *jsonlWriter) Write(_ string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(j.w, "%s\n", body)
	return err
}

func (j *jsonlWriter) WriteError(ip string, err error) error {
	return j.Write(ip, map[string]string{"ip": ip, "error": err.Error()})
}

func (j *jsonlWriter) Close() error { return nil }

// csvWriter emits a fixed header and one row per answer.
type csvWriter struct {
	w      *csv.Writer
	opts   Opts
	cols   []string
	header bool
}

func (c *csvWriter) columns() []string {
	if c.cols != nil {
		return c.cols
	}
	if len(c.opts.Fields) > 0 {
		c.cols = ExpandFields(c.opts.Fields)
	} else {
		c.cols = Columns
	}
	return c.cols
}

func (c *csvWriter) Write(_ string, v any) error {
	rec, err := Flatten(v)
	if err != nil {
		return err
	}
	if err := c.writeHeader(); err != nil {
		return err
	}
	cols := c.columns()
	row := make([]string, len(cols))
	for i, col := range cols {
		// An absent field is an EMPTY cell and a present false one is "false".
		// That distinction is the only way CSV can carry "not in your plan"
		// apart from "checked, and no".
		row[i] = rec[col].Text
	}
	return c.w.Write(row)
}

func (c *csvWriter) WriteError(ip string, err error) error {
	if hErr := c.writeHeader(); hErr != nil {
		return hErr
	}
	cols := c.columns()
	row := make([]string, len(cols))
	for i, col := range cols {
		switch col {
		case "ip":
			row[i] = ip
		case "error":
			row[i] = err.Error()
		}
	}
	return c.w.Write(row)
}

func (c *csvWriter) writeHeader() error {
	if c.header {
		return nil
	}
	c.header = true
	return c.w.Write(c.columns())
}

func (c *csvWriter) Close() error {
	if err := c.writeHeader(); err != nil {
		return err
	}
	c.w.Flush()
	return c.w.Error()
}

// yamlWriter collects and emits once.
//
// YAML has no streamable mapping form that stays valid mid-write, so this one
// format genuinely holds its answers. It is documented as the wrong choice for
// a large run rather than silently made to look like the others.
type yamlWriter struct {
	w       io.Writer
	answers map[string]any
	order   []string
}

func (y *yamlWriter) Write(ip string, v any) error {
	if _, dup := y.answers[ip]; !dup {
		y.order = append(y.order, ip)
	}
	y.answers[ip] = v
	return nil
}

func (y *yamlWriter) WriteError(ip string, err error) error {
	return y.Write(ip, map[string]string{"error": err.Error()})
}

func (y *yamlWriter) Close() error {
	enc := yaml.NewEncoder(y.w)
	enc.SetIndent(2)
	if err := enc.Encode(y.answers); err != nil {
		return err
	}
	return enc.Close()
}

// prettyWriter emits the human block, one per answer.
type prettyWriter struct {
	w     io.Writer
	opts  Opts
	count int
}

func (p *prettyWriter) Write(ip string, v any) error {
	rec, err := Flatten(v)
	if err != nil {
		return err
	}
	if p.count > 0 {
		fmt.Fprintln(p.w)
	}
	p.count++
	return WritePretty(p.w, rec, p.opts)
}

func (p *prettyWriter) WriteError(ip string, err error) error {
	if p.count > 0 {
		fmt.Fprintln(p.w)
	}
	p.count++
	bad := color.New(color.FgRed)
	if !p.opts.Color {
		bad.DisableColor()
	}
	_, wErr := fmt.Fprintf(p.w, "%s\n  %s\n", ip, bad.Sprint(err.Error()))
	return wErr
}

func (p *prettyWriter) Close() error { return nil }

// extraOf is the fields an answer carried that Columns does not name, which is
// what the API growing a field looks like from here.
func extraOf(rec Record) []string {
	known := make(map[string]bool, len(Columns)+len(Objects))
	for _, c := range Columns {
		known[c] = true
	}
	for _, o := range Objects {
		known[o] = true
	}
	out := make([]string, 0)
	for k, c := range rec {
		if c.Present && !known[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// WritePretty renders one answer as a readable block.
//
// Only the fields this plan actually carries are printed. What it does NOT
// carry is named once at the end rather than shown as a wall of blanks - a
// reader who sees no `is_tor` line needs to know that means "not included"
// rather than "not a Tor node", and that is one sentence, not thirty rows.
func WritePretty(w io.Writer, rec Record, opts Opts) error {
	name := color.New(color.FgCyan)
	value := color.New(color.FgGreen)
	yes := color.New(color.FgYellow, color.Bold)
	faint := color.New(color.Faint)
	if !opts.Color {
		for _, c := range []*color.Color{name, value, yes, faint} {
			c.DisableColor()
		}
	}

	fields := rec.Present()
	switch {
	case len(opts.Fields) > 0:
		fields = ExpandFields(opts.Fields)
	case opts.ShowAbsent:
		// Every column, so the ones this plan does not include are rendered as
		// "-" rather than summarised at the end.
		fields = append(append([]string{}, Columns...), extraOf(rec)...)
	}

	width := 0
	for _, f := range fields {
		if len(f) > width {
			width = len(f)
		}
	}

	for _, f := range fields {
		cell := rec[f]
		text := cell.Text
		if !cell.Present {
			if !opts.ShowAbsent && len(opts.Fields) == 0 {
				continue
			}
			text = "-"
		}
		rendered := value.Sprint(text)
		switch {
		case text == "true":
			rendered = yes.Sprint(text)
		case text == "":
			// A present-but-empty detail object says its flag is false.
			rendered = faint.Sprint("{}")
		case !cell.Present:
			rendered = faint.Sprint(text)
		}
		if _, err := fmt.Fprintf(w, "%s  %s\n", name.Sprintf("%-*s", width, f), rendered); err != nil {
			return err
		}
	}

	if opts.ShowAbsent || len(opts.Fields) > 0 {
		return nil
	}
	// Named at the end rather than shown as rows, and summarised rather than
	// enumerated: on the free tier 35 of the 37 fields are absent, and a
	// thirty-five-item list buries the two that answered. The FLAGS are what a
	// reader must not mistake for a negative, so those are named; their detail
	// objects follow from them and are counted.
	if absent := rec.Absent(); len(absent) > 0 {
		flags := make([]string, 0, len(Flags))
		details := 0
		for _, f := range absent {
			if strings.ContainsRune(f, '.') {
				details++
				continue
			}
			flags = append(flags, f)
		}
		note := "not included in this plan (absent, which is not false): "
		switch {
		case len(flags) == 0:
			note += fmt.Sprintf("%d detail field(s)", details)
		case details == 0:
			note += strings.Join(flags, ", ")
		default:
			note += fmt.Sprintf("%s, and %d detail field(s)", strings.Join(flags, ", "), details)
		}
		_, err := fmt.Fprintf(w, "\n%s\n", faint.Sprint(note))
		return err
	}
	return nil
}
