// Package lib holds the CLI's output machinery: how one answer becomes a table
// row, a CSV line, a JSON object or a colored block.
package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Columns is every field an answer can carry, in CSV's order: the address, then
// the flags, then each flag's detail object, then the fields added since.
//
// Fixed and explicit rather than derived by reflection, because this order IS
// the CSV contract. A caller's column indexes must not move because the API
// grew a field, and a new field appearing silently at the end is far easier to
// absorb than one appearing in the middle. The readable block lists each one
// with its object's other fields all the same. Three exceptions were chosen:
// 1.10.0 put is_pubproxy with the other flags, 1.13.0 put each provider_name
// right after its provider, and 1.16.0 put is_bot after is_cdn and bot after
// cdn, the answer's order, each moving every column after it.
//
// vpn.method keeps its column, always empty: no plan is answered with it since
// 2026-10-20, and dropping it would move every column after it.
var Columns = []string{
	"ip",
	"is_bogon",

	"is_vpn",
	"is_hosting",
	"is_relay",
	"is_tor",
	"is_cdn",
	"is_bot",
	"is_resproxy",
	"is_dcproxy",
	"is_mobproxy",
	"is_pubproxy",

	"vpn.provider", "vpn.provider_name", "vpn.last_seen", "vpn.confidence", "vpn.method",
	"hosting.provider", "hosting.provider_name", "hosting.confidence", "hosting.last_seen",
	"relay.provider", "relay.provider_name", "relay.confidence", "relay.last_seen",
	"tor.provider", "tor.provider_name", "tor.confidence", "tor.last_seen",
	"cdn.provider", "cdn.provider_name", "cdn.confidence", "cdn.last_seen",
	"bot.provider", "bot.provider_name", "bot.agents", "bot.categories", "bot.verified", "bot.identity",
	"bot.verified_by", "bot.confidence", "bot.first_seen", "bot.last_seen",
	"resproxy.provider", "resproxy.provider_name", "resproxy.first_seen", "resproxy.last_seen", "resproxy.hits",
	"dcproxy.provider", "dcproxy.provider_name", "dcproxy.first_seen", "dcproxy.last_seen", "dcproxy.hits",
	"mobproxy.provider", "mobproxy.provider_name", "mobproxy.first_seen", "mobproxy.last_seen", "mobproxy.hits",

	"resproxy.hits_days_pct", "resproxy.providers_num",
	"dcproxy.hits_days_pct", "dcproxy.providers_num",
	"mobproxy.hits_days_pct", "mobproxy.providers_num",

	"pubproxy.entry", "pubproxy.exit", "pubproxy.protocols",
	"pubproxy.num_ports", "pubproxy.num_proxies",
	"pubproxy.last_seen", "pubproxy.last_usable",

	"location.continent", "location.continent_name", "location.country", "location.country_name",
	"location.timezone", "location.currency_code", "location.currency_name", "location.currency_symbol",
	"location.flag_emoji", "location.flag_unicode",
	"asn.asn", "asn.name", "asn.domain", "asn.type", "asn.prefix",

	// On a free-plan key's answers alone: false for its trial's lookups with
	// every field, true once they are spent and the answer is Unlimited's.
	"trial_expired",
	// On a paid key's answers alone: false while the answer has every field, true
	// once its organization's overage cap is reached and the answer is Unlimited's.
	"cap_reached",

	// The services, carrier and satellite flags, each detail, and who uses and
	// answers for the address, from 1.15.0. web is one column holding its list.
	"is_anycast", "is_carrier", "is_satellite", "is_ssh", "is_smtp", "is_web", "is_dns_resolver",
	"is_bittorrent",
	"carrier.name", "carrier.mcc", "carrier.mnc", "carrier.country", "carrier.confidence",
	"satellite.provider", "satellite.provider_name", "satellite.orbit", "satellite.segment",
	"satellite.country", "satellite.asn", "satellite.confidence",
	"ssh.port", "ssh.banner", "ssh.host_key_fp", "ssh.session_ok", "ssh.first_seen", "ssh.last_seen",
	"smtp.port", "smtp.banner", "smtp.session_ok", "smtp.first_seen", "smtp.last_seen",
	"web",
	"dns_resolver.port", "dns_resolver.recursion_available", "dns_resolver.rcode",
	"dns_resolver.answer_correct", "dns_resolver.first_seen", "dns_resolver.last_seen",
	"bittorrent.verified", "bittorrent.id_matches_ip", "bittorrent.first_seen", "bittorrent.last_seen",
	"bittorrent.last_reachable", "bittorrent.days_seen", "bittorrent.num_ports",
	"bittorrent.peak_num_ports", "bittorrent.indexer", "bittorrent.clients",
	"location.region", "location.region_code", "location.city", "location.postal_code",
	"location.latitude", "location.longitude", "location.accuracy_radius", "location.geoname_id",
	"location.dma_code", "location.last_changed",
	"asn.country", "asn.rpki_status",
	"company.name", "company.domain", "company.type", "company.country",
	"hostname",
	"abuse.email", "abuse.name", "abuse.phone", "abuse.address", "abuse.country", "abuse.network",
	"abuse.validated", "abuse.unvalidated_since",

	// Only to a lookup that sent a visitor's agent, on a paid plan or the trial,
	// from 1.17.0: the agent it names, its kind, and whether the address bears it out.
	"user_agent.agent", "user_agent.kind", "user_agent.verdict",
}

// retired are columns no answer carries any more, kept so no CSV column moves.
// The readable block leaves them out even under --show-absent, since absent is
// all they can be.
var retired = map[string]bool{"vpn.method": true}

// Flags are the top-level booleans, which is what a summary line shows.
var Flags = []string{
	"is_vpn", "is_hosting", "is_relay", "is_tor",
	"is_cdn", "is_bot", "is_resproxy", "is_dcproxy", "is_mobproxy", "is_pubproxy",
	"is_anycast", "is_carrier", "is_satellite", "is_ssh", "is_smtp", "is_web", "is_dns_resolver",
	"is_bittorrent",
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
	// EmptyObject is a detail object that arrived as {}, which says the flag
	// above it is false. Its Text is "", as a present empty string's is.
	EmptyObject bool
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
	tree, err := asTree(v)
	if err != nil {
		return nil, err
	}
	rec := make(Record, len(Columns))
	walk(rec, "", tree)
	return rec, nil
}

// asTree is an answer as the API sent it: its JSON, decoded into maps. Every
// format reads the answer through it, so each one carries the API's keys and
// nesting whatever Go type the answer arrived in.
func asTree(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	return tree, nil
}

// Served is an answer with the members it left out although its Go type cannot
// say so: sdk-go keeps asn.type a plain string until its next major, so on
// Unlimited it reads "". Its JSON, which every format reads an answer through,
// leaves each of Unserved's dotted paths out, so they print as absent, as any
// field the plan does not include does, while a paid answer's "" stays "".
type Served struct {
	Answer   any
	Unserved []string
}

// MarshalJSON is Answer's JSON less each path in Unserved.
func (s Served) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(s.Answer)
	if err != nil {
		return nil, err
	}
	for _, path := range s.Unserved {
		if raw, err = without(raw, strings.Split(path, ".")); err != nil {
			return nil, err
		}
	}
	return raw, nil
}

// without is the JSON object raw less the member at path, every other member
// kept in its place. Anything but an object, or one without that member, is raw.
func without(raw json.RawMessage, path []string) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if open, err := decoder.Token(); err != nil || open != json.Delim('{') {
		return raw, nil
	}
	var out bytes.Buffer
	out.WriteByte('{')
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		if key == path[0] {
			if len(path) == 1 {
				continue
			}
			if value, err = without(value, path[1:]); err != nil {
				return nil, err
			}
		}
		if out.Len() > 1 {
			out.WriteByte(',')
		}
		encoded, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		out.Write(encoded)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
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
				rec[key] = Cell{Present: true, EmptyObject: true}
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

// Objects are the detail objects, in the order output presents them: each
// flag's, in Flags' order, then the location and the AS, which every answer
// carries whole, the company and abuse contact, and the visitor's agent.
var Objects = []string{
	"vpn", "hosting", "relay", "tor", "cdn", "bot", "resproxy", "dcproxy", "mobproxy",
	"pubproxy", "carrier", "satellite", "ssh", "smtp", "dns_resolver", "bittorrent",
	"location", "asn", "company", "abuse", "user_agent",
}

// Present lists the fields this answer actually carries, in display order:
// the address, the flags, then each detail object.
//
// A detail object that arrived EMPTY is listed by its own name rather than by
// its fields, because that is what it is - `"vpn": {}` says the flag above it is
// false, and expanding it into four blank rows would say something else.
func (r Record) Present() []string {
	return r.listed(false, false)
}

// All lists what Present does and, each in its place among them, the fields
// this answer does not carry, which is what --show-absent prints. user_agent is
// among them only when the lookup sent an agent: without one, no plan answers
// it, and listing it would say the plan left it out.
func (r Record) All(agentSent bool) []string {
	return r.listed(true, agentSent)
}

// Selected lists what a --field selection names, an object as its fields but
// a retired one the answer lacks, which only naming it alone shows; a detail
// object which arrived empty is listed once by its own name, as Present lists it.
func (r Record) Selected(fields []string) []string {
	out := make([]string, 0, len(fields))
	seen := make(map[string]bool)
	for _, named := range fields {
		for _, f := range ExpandFields([]string{named}) {
			if retired[f] && f != named && !r[f].Present {
				continue
			}
			obj, _, nested := strings.Cut(f, ".")
			if !nested || !r[obj].EmptyObject {
				out = append(out, f)
				continue
			}
			if !seen[obj] {
				out = append(out, obj)
				seen[obj] = true
			}
		}
	}
	return out
}

func (r Record) listed(absent, agentSent bool) []string {
	out := make([]string, 0, len(Columns))
	seen := make(map[string]bool, len(Columns))
	listable := func(k string) bool {
		if r[k].Present {
			return true
		}
		return absent && !retired[k] && (agentSent || !strings.HasPrefix(k, "user_agent."))
	}
	emit := func(k string) {
		if listable(k) && !seen[k] {
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
		if r[obj].Present {
			emit(obj)
			continue
		}
		for _, col := range Columns {
			if strings.HasPrefix(col, obj+".") {
				emit(col)
			}
		}
	}
	// A top-level field that is neither a flag nor an object, as hostname, and
	// last trial_expired and cap_reached, which say what the answer is rather
	// than what the address is.
	for _, col := range Columns {
		if !strings.Contains(col, ".") && col != "trial_expired" && col != "cap_reached" {
			emit(col)
		}
	}
	emit("trial_expired")
	emit("cap_reached")
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

// selectFields keeps what a --field selection names, nested as in the full
// answer: `is_vpn` keeps that flag, `vpn` the whole detail object as it was
// served, and `vpn.provider` that object holding its provider alone.
//
// A field the answer does not carry stays out rather than appearing as null,
// and a detail object that arrived empty stays `{}` whichever of its fields
// was named, because that is the answer saying its flag is false.
func selectFields(tree map[string]any, fields []string) map[string]any {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		name, sub, nested := strings.Cut(f, ".")
		v, ok := tree[name]
		if !ok {
			continue
		}
		if !nested {
			out[name] = v
			continue
		}
		obj, ok := v.(map[string]any)
		if !ok {
			continue
		}
		kept, ok := out[name].(map[string]any)
		if !ok {
			kept = map[string]any{}
			out[name] = kept
		}
		if sv, ok := obj[sub]; ok {
			kept[sub] = sv
		}
	}
	return out
}
