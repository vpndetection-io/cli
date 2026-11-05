package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	vpndetection "github.com/vpndetection-io/sdk-go/v5"
)

// The API reads a license whose term has not begun as `expired`, beside the
// `starts` that says otherwise, so the term column has to look at both.
func TestLicenseTerm(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	day := func(s string) *time.Time {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return &d
	}
	for _, c := range []struct {
		name string
		d    vpndetection.Database
		want string
	}{
		{"never bought", vpndetection.Database{Standing: vpndetection.StandingUnlicensed}, ""},
		{"not started", vpndetection.Database{Standing: vpndetection.StandingExpired,
			Starts: day("2026-11-01"), Expires: day("2027-11-01")}, "starts 2026-11-01"},
		{"ended", vpndetection.Database{Standing: vpndetection.StandingExpired,
			Starts: day("2025-10-01"), Expires: day("2026-10-01")}, "lapsed"},
		{"hard stop", vpndetection.Database{Standing: vpndetection.StandingLicensed, InTerm: true,
			Starts: day("2026-01-01"), Expires: day("2027-01-01")}, "until 2027-01-01"},
		{"rolling", vpndetection.Database{Standing: vpndetection.StandingLicensed, InTerm: true,
			Starts: day("2026-01-01"), RenewsAt: day("2027-01-01")}, "renews 2027-01-01"},
		{"no term", vpndetection.Database{Standing: vpndetection.StandingLicensed, InTerm: true,
			Starts: day("2026-01-01")}, ""},
	} {
		if got := licenseTerm(c.d, now); got != c.want {
			t.Errorf("%s: licenseTerm = %q, want %q", c.name, got, c.want)
		}
	}
}

// A database API whose live build moves on cue: Checksums answers sums[0] until published, then sums[1], and
// DownloadFile writes file, then runs onDownload, as a build published during the transfer would.
type fakeFiles struct {
	file       []byte
	sums       []string
	published  bool
	onDownload func(f *fakeFiles)
}

func (f *fakeFiles) Download(context.Context, string, vpndetection.Format, io.Writer) (int64, error) {
	return 0, nil
}

func (f *fakeFiles) DownloadFile(_ context.Context, _ string, _ vpndetection.Format, path string) (int64, error) {
	if err := os.WriteFile(path, f.file, 0o644); err != nil {
		return 0, err
	}
	if f.onDownload != nil {
		f.onDownload(f)
	}
	return int64(len(f.file)), nil
}

func (f *fakeFiles) Checksums(context.Context, string, vpndetection.Format) (*vpndetection.Checksums, error) {
	if f.published {
		return &vpndetection.Checksums{SHA256: f.sums[1]}, nil
	}
	return &vpndetection.Checksums{SHA256: f.sums[0]}, nil
}

func sum256(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// The download is of the build live when it starts; a build published during it moves the checksum on.
func TestDownloadVerifiesAgainstTheChecksumTakenBeforeIt(t *testing.T) {
	a, b := []byte("build a"), []byte("build b")
	f := &fakeFiles{file: a, sums: []string{sum256(a), sum256(b)}, onDownload: func(f *fakeFiles) { f.published = true }}
	out := filepath.Join(t.TempDir(), "x.csv.gz")
	if err := dbDownload(context.Background(), f, "x_v1", "csvgz", out, false, false); err != nil {
		t.Fatalf("a good file read as bad: %v", err)
	}
}

// A build published between the checksum and the download's start: the download is of it, and the checksum
// asked once more names it.
func TestDownloadAsksAgainWhenABuildWentLiveBeforeItStarted(t *testing.T) {
	a, b := []byte("build a"), []byte("build b")
	f := &fakeFiles{file: b, sums: []string{sum256(a), sum256(b)}, onDownload: func(f *fakeFiles) { f.published = true }}
	out := filepath.Join(t.TempDir(), "x.csv.gz")
	if err := dbDownload(context.Background(), f, "x_v1", "csvgz", out, false, false); err != nil {
		t.Fatalf("the newer build's file read as bad: %v", err)
	}
}

func TestDownloadRefusesAFileNoChecksumDescribes(t *testing.T) {
	a := []byte("build a")
	f := &fakeFiles{file: []byte("corrupt"), sums: []string{sum256(a), sum256(a)}}
	out := filepath.Join(t.TempDir(), "x.csv.gz")
	if err := dbDownload(context.Background(), f, "x_v1", "csvgz", out, false, false); err == nil {
		t.Fatal("a file no checksum describes passed")
	}
}
