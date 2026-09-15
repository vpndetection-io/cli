package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	vpndetection "github.com/vpndetection-io/sdk-go/v5"

	"github.com/vpndetection-io/cli/lib"
)

func printHelpWhoami() {
	fmt.Printf(
		`Usage: %[1]s whoami [<opts>]

Aliases: entitlement

Description:
  Which credential this machine uses, the plan behind it, and how much of the
  allowance has been spent.

  Usage counts against the anniversary of your subscription, not the calendar
  month and not the billing period, and it is the same number a lookup is
  gated on. It can lag a few seconds behind what you have just sent.

Examples:
  $ %[1]s whoami
  $ %[1]s entitlement --json
  $ %[1]s --session work whoami

Options:
  --json, -j
    output JSON instead of the readable block.
  --help, -h
    show help.
`, progBase)
}

func cmdWhoami() error {
	globalFlags()
	lookupFlags()
	resolve := formatFlags()
	parseSubFlags()

	if fHelp {
		printHelpWhoami()
		return nil
	}
	opts, err := resolve()
	if err != nil {
		return err
	}

	key, source := gConfig.ResolveKey()
	api := gConfig.ResolveBaseURL()
	if api == "" {
		api = vpndetection.DefaultBaseURL
	}

	if key == "" {
		fmt.Println("not authenticated")
		fmt.Printf("\nLookups still work: the free tier answers 'ip' and 'is_vpn'.\n")
		fmt.Printf("Run `%s login` to use a key, or `%s signup` to create an account.\n", progBase, progBase)
		return nil
	}

	client, err := NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	acct, err := client.api.MyEntitlement(context.Background())
	if err != nil {
		return explain(err)
	}

	if opts.format == lib.FormatJSON || opts.format == lib.FormatJSONL {
		return emitJSON(acct)
	}

	// The credential first, because "which key am I even using" is the question
	// that brings most people here.
	fmt.Printf("key          %s\n", maskKey(key))
	fmt.Printf("from         %s\n", source)
	if name := gConfig.ActiveSessionName(); name != "" && gConfig.Sessions[name] != nil {
		fmt.Printf("session      %s\n", name)
	}
	fmt.Printf("api          %s\n", api)
	fmt.Printf("org          %s\n", acct.OrgID)
	if len(acct.Apikey.AllowedCidrs) > 0 {
		fmt.Printf("allowed from %s\n", strings.Join(acct.Apikey.AllowedCidrs, ", "))
	}
	if acct.Apikey.Expires != nil {
		fmt.Printf("expires      %s\n", acct.Apikey.Expires.Format(time.RFC3339))
	}

	fmt.Printf("\nplan         %s\n", acct.Plan.Key)
	fmt.Printf("fields       %s tier\n", acct.Plan.Tier)

	fmt.Printf("\nused         %s of %s\n",
		humanCount(acct.Usage.Requests), quotaText(acct.Usage.Quota))
	if acct.Usage.Quota > 0 {
		fmt.Printf("             %s\n", usageBar(acct.Usage.Requests, acct.Usage.Quota))
	}
	// Null means never, which is NOT zero - an uncapped paid plan has no stop.
	if acct.Usage.HardLimit == nil {
		fmt.Printf("hard limit   none; requests above the quota are billed as overage\n")
	} else {
		fmt.Printf("hard limit   %s\n", humanCount(*acct.Usage.HardLimit))
	}
	fmt.Printf("resets       %s (%s)\n",
		acct.Usage.WindowEnd.Format(time.RFC3339), until(acct.Usage.WindowEnd))
	return nil
}

// usageBar draws consumption as a proportion, which is the thing a person
// actually reads off this command.
func usageBar(used, quota int64) string {
	const width = 40
	pct := float64(used) / float64(quota)
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * width)
	return fmt.Sprintf("[%s%s] %.1f%%",
		strings.Repeat("#", filled), strings.Repeat(" ", width-filled),
		float64(used)/float64(quota)*100)
}

// quotaText renders an allowance, naming a plan that includes none rather than
// printing a bare 0 the reader has to interpret.
func quotaText(quota int64) string {
	if quota <= 0 {
		return "no included allowance"
	}
	return humanCount(quota)
}

// humanCount groups a request count, because seven digits are unreadable.
func humanCount(n int64) string {
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

// until renders how long is left, for a reader who does not want to subtract
// dates in their head.
func until(t time.Time) string {
	d := time.Until(t)
	switch {
	case d <= 0:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("in %dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("in %dh", int(d.Hours()))
	default:
		return fmt.Sprintf("in %dd", int(d.Hours()/24))
	}
}
