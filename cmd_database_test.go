package main

import (
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
