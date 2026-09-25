// Package lib holds the CLI's output machinery: how one answer becomes a table
// row, a CSV line, a JSON object or a colored block.
package lib

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Columns is every field an answer can carry, in the order output presents
// them: the address, then the flags, then each flag's detail object.
//
// Fixed and explicit rather than derived by reflection, because this order IS
// the CSV contract. A caller's column indexes must not move because the API
// grew a field, and a new field appearing silently at the end is far easier to
// absorb than one appearing in the middle.
var Columns = []string{
	"ip",
	"is_bogon",

	"is_vpn",
	"is_hosting",
	"is_relay",
	"is_tor",
	"is_cdn",
	"is_resproxy",
	"is_dcproxy",
	"is_mobproxy",

	"vpn.provider", "vpn.last_seen", "vpn.confidence", "vpn.method",
	"hosting.provider", "hosting.confidence", "hosting.last_seen",
	"relay.provider", "relay.confidence", "relay.last_seen",
	"tor.provider", "tor.confidence", "tor.last_seen",
	"cdn.provider", "cdn.confidence", "cdn.last_seen",
	"resproxy.provider", "resproxy.first_seen", "resproxy.last_seen", "resproxy.hits",
	"dcproxy.provider", "dcproxy.first_seen", "dcproxy.last_seen", "dcproxy.hits",
	"mobproxy.provider", "mobproxy.first_seen", "mobproxy.last_seen", "mobproxy.hits",
}

// Flags are the top-level booleans, which is what a summary line shows.
var Flags = []string{
	"is_vpn", "is_hosting", "is_relay", "is_tor",
	"is_cdn", "is_resproxy", "is_dcproxy", "is_mobproxy",
}

// Cell is one field of one answer.
//
// Present is the whole point and is not the same as an empty Text. A field the
// plan does not include is ABSENT; a field it does include and that is false is
// PRESENT and false. Collapsing the two is the mistake this product's API
// design exists to prevent, and it would be a shame to reintroduce it in the
// client that prints the answer.
type Cell struct {
	Text    string
	Present bool
}

// Record is one answer flattened to dotted field names.
type Record map[string]Cell

// Flatten converts an answer into dotted fields.
//
// It goes through JSON rather than reflecting over the struct, so a field the
// SDK gains appears here with no change, and absence falls out of the wire
// shape exactly as the API sent it: an optional field the plan excludes is a
// missing key, not a zero value.
func Flatten(v any) (Record, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	rec := make(Record, len(Columns))
	walk(rec, "", tree)
	return rec, nil
}

// walk flattens nested objects into dotted keys.
func walk(rec Record, prefix string, tree map[string]any) {
	for k, v := range tree {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if sub, ok := v.(map[string]any); ok {
			// An EMPTY detail object is meaningful on its own: it says the flag
			// above it is false. Recorded as a present-but-empty cell so it can
			// be told apart from a detail object the plan does not include.
			if len(sub) == 0 {
				rec[key] = Cell{Present: true}
				continue
			}
			walk(rec, key, sub)
			continue
		}
		rec[key] = Cell{Text: scalar(v), Present: true}
	}
}

// scalar renders a JSON value as the text a table or CSV shows.
func scalar(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		// JSON has one number type; render whole numbers without a decimal
		// point so a hit count reads as 42 rather than 42.000000.
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

// Get reads one dotted field.
func (r Record) Get(field string) Cell { return r[field] }

// Objects are the detail objects, in the order output presents them. Each one
// belongs to the flag of the same name.
var Objects = []string{
	"vpn", "hosting", "relay", "tor", "cdn", "resproxy", "dcproxy", "mobproxy",
}

// Present lists the fields this answer actually carries, in display order:
// the address, the flags, then each detail object.
//
// A detail object that arrived EMPTY is listed by its own name rather than by
// its fields, because that is what it is - `"vpn": {}` says the flag above it is
// false, and expanding it into four blank rows would say something else.
func (r Record) Present() []string {
	out := make([]string, 0, len(r))
	seen := make(map[string]bool, len(r))
	emit := func(k string) {
		if c, ok := r[k]; ok && c.Present && !seen[k] {
			out = append(out, k)
			seen[k] = true
		}
	}

	emit("ip")
	emit("is_bogon")
	for _, f := range Flags {
		emit(f)
	}
	for _, obj := range Objects {
		if c, ok := r[obj]; ok && c.Present {
			emit(obj)
			continue
		}
		for _, col := range Columns {
			if strings.HasPrefix(col, obj+".") {
				emit(col)
			}
		}
	}
	// A field the API grew since this build must still be printed, or the CLI
	// silently hides what the customer is paying for.
	extra := make([]string, 0)
	for k, c := range r {
		if !seen[k] && c.Present {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}

// ValidateFields refuses a --field selection naming something that could never
// exist, so a typo is an error rather than a column of blanks.
//
// A field that is merely ABSENT from this plan is accepted: it is a real field,
// and printing it empty is the honest answer.
func ValidateFields(fields []string) error {
	known := make(map[string]bool, len(Columns))
	for _, col := range Columns {
		known[col] = true
		// A bare object name selects the whole object.
		if i := strings.IndexByte(col, '.'); i > 0 {
			known[col[:i]] = true
		}
	}
	var bad []string
	for _, f := range fields {
		if !known[f] {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf("unknown field(s): %s", strings.Join(bad, ", "))
}

// ExpandFields turns a selection into concrete columns, so naming an object
// selects every field inside it.
func ExpandFields(fields []string) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.ContainsRune(f, '.') {
			out = append(out, f)
			continue
		}
		matched := false
		for _, col := range Columns {
			if strings.HasPrefix(col, f+".") {
				out = append(out, col)
				matched = true
			}
		}
		if !matched {
			out = append(out, f)
		}
	}
	return out
}
