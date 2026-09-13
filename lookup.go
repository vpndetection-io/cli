package main

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/pflag"

	iputil "github.com/mslmio/libgo-iputil"
	vpndetection "github.com/vpndetection-io/sdk-go/v4"

	"github.com/vpndetection-io/cli/lib"
)

// chunkSize is how many addresses are looked up per batch.
//
// The input is unbounded - a single `8.0.0.0/8` argument is 16.7 million
// addresses - so it is consumed in chunks rather than collected. Each chunk is
// one SDK batch, whose answers are emitted and dropped before the next chunk is
// read, which puts a fixed ceiling on memory whatever was typed.
const chunkSize = 10_000

// lookupOpts is what the lookup commands were asked for.
type lookupOpts struct {
	// format is the explicit --format, or "" to choose by answer count.
	format lib.Format
	// forceBulk skips the single-answer form even for one address, which is
	// what the explicit `bulk` subcommand wants so its output shape is
	// predictable in a script.
	forceBulk  bool
	fields     []string
	showAbsent bool
	// input controls where addresses are read from.
	input iputil.Opts
}

// runLookup reads addresses from args and standard input and answers them.
//
// One address gets the readable block; more than one gets the machine format.
// That is decided by how many addresses the input RESOLVES to, not by how many
// arguments there were: a single `10.0.0.0/30` is four addresses and a single
// `10.0.0.1/32` is one.
func runLookup(ctx context.Context, args []string, opts lookupOpts) error {
	client, err := NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	var (
		chunk   []string
		writer  lib.Writer
		flushed bool
	)

	// Buffered until the shape is known, because the format depends on the
	// answer count and nothing may be written before that is settled. The wait
	// is bounded by chunkSize, not by the size of the input.
	openWriter := func(single bool) {
		format := opts.format
		if format == "" {
			format = lib.FormatJSON
			if single {
				format = lib.Format(gConfig.Format)
				if format == "" {
					format = lib.FormatPretty
				}
			}
		}
		writer = lib.NewWriter(os.Stdout, format, lib.Opts{
			Fields:     opts.fields,
			ShowAbsent: opts.showAbsent,
			Color:      !color.NoColor,
		})
	}

	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		if writer == nil {
			openWriter(!flushed && len(chunk) == 1 && !opts.forceBulk)
		}
		flushed = true
		answers, err := client.LookupBatch(ctx, chunk)
		if err != nil {
			return explain(err)
		}
		for _, ip := range chunk {
			answer, ok := answers[ip]
			if !ok {
				continue
			}
			if answer.Err != nil {
				if err := writer.WriteError(ip, answer.Err); err != nil {
					return err
				}
				continue
			}
			if err := writer.Write(ip, answer.Result); err != nil {
				return err
			}
		}
		chunk = chunk[:0]
		return nil
	}

	walkErr := iputil.WalkAddrs(args, opts.input, func(addr netip.Addr) error {
		chunk = append(chunk, addr.String())
		if len(chunk) < chunkSize {
			return nil
		}
		return flush()
	})
	if walkErr != nil {
		return walkErr
	}
	if err := flush(); err != nil {
		return err
	}

	if writer == nil {
		return errNoInput
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return nil
}

// errNoInput is returned when the command found nothing to look up.
var errNoInput = errors.New("no addresses in input")

// lookupOne answers a single address with no streaming machinery, for the
// commands that already know they have exactly one.
func lookupOne(ctx context.Context, ip string, opts lookupOpts) error {
	client, err := NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	result, err := client.Lookup(ctx, ip)
	if err != nil {
		return explain(err)
	}
	return writeOne(ip, result, opts)
}

// writeOne renders one answer in the chosen format.
func writeOne(ip string, result *vpndetection.Result, opts lookupOpts) error {
	format := opts.format
	if format == "" {
		format = lib.Format(gConfig.Format)
		if format == "" {
			format = lib.FormatPretty
		}
	}
	w := lib.NewWriter(os.Stdout, format, lib.Opts{
		Fields:     opts.fields,
		ShowAbsent: opts.showAbsent,
		Color:      !color.NoColor,
	})
	if err := w.Write(ip, result); err != nil {
		return err
	}
	return w.Close()
}

// formatFlags registers the output options the lookup commands share, and
// returns a function resolving them into a lookupOpts.
func formatFlags() func() (lookupOpts, error) {
	var (
		fFields     []string
		fFormat     string
		fJSON       bool
		fJSONL      bool
		fCSV        bool
		fYAML       bool
		fPretty     bool
		fShowAbsent bool
	)
	pflag.StringSliceVarP(&fFields, "field", "f", nil, "only these fields, comma separated.")
	pflag.StringVar(&fFormat, "format", "", "output format: pretty, json, jsonl, csv, yaml.")
	pflag.BoolVarP(&fJSON, "json", "j", false, "output JSON.")
	pflag.BoolVar(&fJSONL, "jsonl", false, "output one JSON object per line.")
	pflag.BoolVarP(&fCSV, "csv", "c", false, "output CSV.")
	pflag.BoolVarP(&fYAML, "yaml", "y", false, "output YAML.")
	pflag.BoolVarP(&fPretty, "pretty", "p", false, "output the readable block.")
	pflag.BoolVar(&fShowAbsent, "show-absent", false, "show fields this plan does not include.")

	return func() (lookupOpts, error) {
		opts := lookupOpts{
			fields:     fFields,
			showAbsent: fShowAbsent,
			input:      iputil.ListOpts,
		}
		// The shorthands are conveniences for --format, and giving two of them
		// is a contradiction rather than a precedence question.
		chosen := map[string]bool{}
		if fJSON {
			chosen["json"] = true
		}
		if fJSONL {
			chosen["jsonl"] = true
		}
		if fCSV {
			chosen["csv"] = true
		}
		if fYAML {
			chosen["yaml"] = true
		}
		if fPretty {
			chosen["pretty"] = true
		}
		if fFormat != "" {
			chosen[fFormat] = true
		}
		if len(chosen) > 1 {
			return opts, fmt.Errorf("pick one output format, not %d", len(chosen))
		}
		for name := range chosen {
			format, err := lib.ParseFormat(name)
			if err != nil {
				return opts, err
			}
			opts.format = format
		}
		if err := lib.ValidateFields(opts.fields); err != nil {
			return opts, err
		}
		return opts, nil
	}
}
