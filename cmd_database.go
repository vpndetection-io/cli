package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/pflag"
	vpndetection "github.com/vpndetection-io/sdk-go/v3"
)

func printHelpDatabase() {
	fmt.Printf(
		`Usage: %[1]s database <cmd> [<opts>] [<args>]
       %[1]s db <cmd> [<opts>] [<args>]

Description:
  The licensed datasets: the same classifications the API answers per address,
  published as files you host yourself.

  Access is granted by contract rather than bought self-serve, and needs a key
  carrying the 'db.download' scope. '%[1]s database list' shows what your
  organization holds.

Commands:
  list                 datasets your organization is licensed for.
  metadata <id>        what is inside one: columns, row count, sizes, build date.
  checksum <id>        digests of one published file.
  download <id> [out]  fetch it, verifying the checksum.
  url <id>             print a time-limited download URL, for curl or wget.
  downloads            your organization's recent download attempts.

Examples:
  $ %[1]s db list
  $ %[1]s db metadata vpn_ip_v1
  $ %[1]s db download vpn_ip_v1
  $ %[1]s db download cdn_ip_v1 --format mmdb cdn.mmdb
  $ curl -fL "$(%[1]s db url vpn_ip_v1)" -o vpn_ip_v1.csv.gz

Options:
  --format <csvgz | mmdb>
    which published file. Default: csvgz. The provider catalogues are keyed by
    provider rather than by address, so they have no mmdb.
  --json, -j
    output JSON instead of a table.
  --help, -h
    show help.
`, progBase)
}

func cmdDatabase() error {
	var (
		fFormat   string
		fJSON     bool
		fLimit    int
		fStdout   bool
		fNoVerify bool
	)
	globalFlags()
	lookupFlags()
	pflag.StringVar(&fFormat, "format", string(vpndetection.FormatCSVGZ), "which published file.")
	pflag.BoolVarP(&fJSON, "json", "j", false, "output JSON.")
	pflag.IntVar(&fLimit, "limit", 0, "how many download records to show.")
	pflag.BoolVar(&fStdout, "stdout", false, "write the download to standard output.")
	pflag.BoolVar(&fNoVerify, "no-verify", false, "skip the checksum check after downloading.")
	args := parseSubFlags()

	if fHelp || len(args) == 0 {
		printHelpDatabase()
		return nil
	}

	client, err := NewClient()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.requireKey("the database commands"); err != nil {
		return err
	}

	// No timeout on the parent: a download is bounded by its own transfer, not
	// by a deadline that would kill a multi-gigabyte fetch part way through.
	ctx := context.Background()
	db := client.Database()

	switch strings.ToLower(args[0]) {
	case "list", "ls":
		return dbList(ctx, db, fJSON)
	case "metadata", "meta":
		if len(args) != 2 {
			return errors.New("usage: database metadata <id>")
		}
		return dbMetadata(ctx, db, args[1], fJSON)
	case "checksum", "checksums":
		if len(args) != 2 {
			return errors.New("usage: database checksum <id>")
		}
		return dbChecksum(ctx, db, args[1], fFormat, fJSON)
	case "url":
		if len(args) != 2 {
			return errors.New("usage: database url <id>")
		}
		return dbURL(ctx, db, args[1], fFormat)
	case "download", "dl":
		if len(args) < 2 || len(args) > 3 {
			return errors.New("usage: database download <id> [<output>]")
		}
		out := ""
		if len(args) == 3 {
			out = args[2]
		}
		return dbDownload(ctx, db, args[1], fFormat, out, fStdout, fNoVerify)
	case "downloads", "history":
		return dbDownloads(ctx, db, fLimit, fJSON)
	default:
		printHelpDatabase()
		return fmt.Errorf("%q is not a database subcommand", args[0])
	}
}

func dbList(ctx context.Context, db *vpndetection.DatabaseAPI, asJSON bool) error {
	items, err := db.List(ctx)
	if err != nil {
		return explain(err)
	}
	if asJSON {
		return emitJSON(items)
	}
	if len(items) == 0 {
		fmt.Println("no datasets licensed to this organization")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tLICENCE\tSTANDING\tTERM")
	for _, d := range items {
		for _, v := range d.Versions {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				v.ID, d.Name, d.LicenseType, d.Standing, licenceTerm(d))
		}
	}
	return w.Flush()
}

// licenceTerm renders when a licence ends, which is the one thing a holder has to act
// on. An empty string means there is nothing to act on.
func licenceTerm(d vpndetection.Database) string {
	switch {
	case !d.InTerm:
		return "lapsed"
	case d.Expires != nil:
		return "until " + d.Expires.Format("2006-01-02")
	case d.RenewsAt != nil:
		return "renews " + d.RenewsAt.Format("2006-01-02")
	default:
		return ""
	}
}

func dbMetadata(ctx context.Context, db *vpndetection.DatabaseAPI, id string, asJSON bool) error {
	meta, err := db.Metadata(ctx, id)
	if err != nil {
		return explain(err)
	}
	if asJSON {
		return emitJSON(meta)
	}
	return emitJSON(meta)
}

func dbChecksum(ctx context.Context, db *vpndetection.DatabaseAPI, id, format string, asJSON bool) error {
	sums, err := db.Checksums(ctx, id, vpndetection.Format(format))
	if err != nil {
		return explain(err)
	}
	if asJSON {
		return emitJSON(sums)
	}
	for _, pair := range [][2]string{
		{"md5", sums.MD5}, {"sha1", sums.SHA1},
		{"sha256", sums.SHA256}, {"sha512", sums.SHA512},
	} {
		// A digest the exporter did not write is an empty string, and printing
		// an empty row would read as a digest of nothing.
		if pair[1] != "" {
			fmt.Printf("%-7s %s\n", pair[0], pair[1])
		}
	}
	return nil
}

func dbURL(ctx context.Context, db *vpndetection.DatabaseAPI, id, format string) error {
	url, err := db.DownloadURL(ctx, id, vpndetection.Format(format))
	if err != nil {
		return explain(err)
	}
	fmt.Println(url)
	return nil
}

func dbDownloads(ctx context.Context, db *vpndetection.DatabaseAPI, limit int, asJSON bool) error {
	items, err := db.Downloads(ctx, limit)
	if err != nil {
		return explain(err)
	}
	if asJSON {
		return emitJSON(items)
	}
	if len(items) == 0 {
		fmt.Println("no downloads recorded")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WHEN\tDATASET\tFORMAT\tOUTCOME\tSIZE")
	for _, d := range items {
		size := ""
		if d.Bytes != nil {
			size = humanBytes(int64(*d.Bytes))
		}
		name := d.DatasetID
		if d.Sample {
			name += " (sample)"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			d.Created.Format(time.RFC3339), name, d.Format, d.Outcome, size)
	}
	return w.Flush()
}

func dbDownload(
	ctx context.Context, db *vpndetection.DatabaseAPI,
	id, format, out string, toStdout, noVerify bool,
) error {
	f := vpndetection.Format(format)

	if toStdout {
		// Nothing to verify against: the bytes are gone as they arrive, and
		// buffering a multi-gigabyte file to hash it would defeat the point.
		n, err := db.Download(ctx, id, f, os.Stdout)
		if err != nil {
			return explain(err)
		}
		fmt.Fprintf(os.Stderr, "%s written\n", humanBytes(n))
		return nil
	}

	if out == "" {
		out = id + "." + extensionFor(f)
	}

	fmt.Fprintf(os.Stderr, "downloading %s (%s) to %s...\n", id, format, out)
	n, err := db.DownloadFile(ctx, id, f, out)
	if err != nil {
		return explain(err)
	}
	fmt.Fprintf(os.Stderr, "%s written\n", humanBytes(n))

	if noVerify {
		return nil
	}
	sums, err := db.Checksums(ctx, id, f)
	if err != nil {
		// The file is on disk and intact as far as we know; failing the command
		// over an unavailable checksum would throw away a good download.
		fmt.Fprintf(os.Stderr, "warn: could not fetch checksums to verify: %v\n", explain(err))
		return nil
	}
	if sums.SHA256 == "" {
		fmt.Fprintln(os.Stderr, "warn: no sha256 published for this file; not verified")
		return nil
	}
	got, err := sha256File(out)
	if err != nil {
		return err
	}
	if got != sums.SHA256 {
		return fmt.Errorf(
			"checksum mismatch: got %s, expected %s\n%s is not the published file and should be deleted",
			got, sums.SHA256, out)
	}
	fmt.Fprintln(os.Stderr, "sha256 verified")
	return nil
}

// extensionFor names the file a format produces.
func extensionFor(f vpndetection.Format) string {
	if f == vpndetection.FormatMMDB {
		return "mmdb"
	}
	return "csv.gz"
}

// sha256File digests a file without holding it in memory.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// emitJSON prints a value indented, which is how this API's own responses are
// served and how a person reads one in a terminal.
func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
