package main

import (
	"strings"
	"testing"
)

// The bucket is what stops one key's answers being served to another's.
//
// Which fields an answer carries is decided by the plan behind the key, so a
// cache keyed on the address alone hands a max-tier caller a free-tier answer,
// which reads as "not flagged" rather than "not included". That is the exact
// trap this product's absent-versus-false design exists to prevent, and the
// reference CLI this one follows has it.
func TestCacheBucketSeparatesCredentials(t *testing.T) {
	const host = "https://api.vpndetection.io"

	free := cacheBucket("key-free", host)
	max := cacheBucket("key-max", host)
	if free == max {
		t.Fatalf("two keys share a bucket: %q", free)
	}

	// Same key, same bucket, or the cache never hits.
	if cacheBucket("key-free", host) != free {
		t.Error("the bucket is not stable for one key")
	}

	// Different API, different bucket: two deployments answer differently for
	// the same address under the same key.
	if cacheBucket("key-free", "https://api.example.com") == free {
		t.Error("two deployments share a bucket")
	}

	// An unauthenticated run has its own bucket rather than borrowing one.
	if cacheBucket("", host) == free {
		t.Error("unauthenticated shares a bucket with a key")
	}

	// The key must not be recoverable from the bucket name, which is written to
	// disk in the clear.
	for _, b := range []string{free, max} {
		if strings.Contains(b, "key-free") || strings.Contains(b, "key-max") {
			t.Errorf("the bucket name leaks the key: %q", b)
		}
	}
}

// The host is what distinguishes deployments, so the bucket has to survive a
// base URL given with a path or a trailing slash.
func TestCacheBucketNormalisesBaseURL(t *testing.T) {
	a := cacheBucket("k", "https://api.example.com")
	b := cacheBucket("k", "https://api.example.com/")
	if a != b {
		t.Errorf("a trailing slash changed the bucket: %q vs %q", a, b)
	}
	if cacheBucket("k", "") == cacheBucket("k", "https://api.example.com") {
		t.Error("the default and another deployment share a bucket")
	}
}
