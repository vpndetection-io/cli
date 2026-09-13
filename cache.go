package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	vpndetection "github.com/vpndetection-io/sdk-go/v4"
	"go.etcd.io/bbolt"
)

// Cache is the on-disk lookup cache.
//
// It exists because a CLI process is short-lived: the SDK's own cache helps
// within one `bulk` run and is gone by the next command, so without this
// looking up the same address twice costs two requests and two units of quota.
//
// Entries are bucketed by CREDENTIAL, not just by address. Which fields an
// answer carries is decided by the plan behind the key, so one bucket shared
// across keys hands a max-tier caller a free-tier answer it would read as "not
// flagged" - the absent-versus-false trap, served from our own disk. The bucket
// name is a hash of the key plus the API host; the key itself never lands on
// disk here.
type Cache struct {
	db     *bbolt.DB
	bucket []byte
	ttl    time.Duration
}

// entry is one cached answer.
type entry struct {
	// Created is when it was fetched, for the TTL.
	Created time.Time `json:"c"`
	// Result is the answer exactly as the SDK returned it.
	Result *vpndetection.Result `json:"r"`
}

// CachePath is the cache database file.
func CachePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cache.boltdb"), nil
}

// OpenCache opens the cache for one credential.
//
// A failure to open is NOT fatal to the caller: a cache is an optimization, and
// a locked or corrupt database should slow a lookup down rather than stop it.
// Callers take a nil *Cache and carry on.
func OpenCache(key, baseURL string, ttl time.Duration) (*Cache, error) {
	path, err := CachePath()
	if err != nil {
		return nil, err
	}
	// Timeout rather than block: bbolt takes an exclusive flock, so a long
	// `bulk` in another terminal would otherwise hang this one indefinitely.
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	bucket := []byte(cacheBucket(key, baseURL))
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &Cache{db: db, bucket: bucket, ttl: ttl}, nil
}

// cacheBucket names the partition an answer for this credential belongs in.
func cacheBucket(key, baseURL string) string {
	host := "api.vpndetection.io"
	if baseURL != "" {
		if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
			host = u.Host
		} else {
			host = baseURL
		}
	}
	return keyFingerprint(key) + "|" + host
}

// Close releases the database.
func (c *Cache) Close() error {
	if c == nil {
		return nil
	}
	return c.db.Close()
}

// Get returns a cached answer, or nil when there is none or it has expired.
func (c *Cache) Get(ip string) *vpndetection.Result {
	if c == nil {
		return nil
	}
	var found *entry
	_ = c.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(c.bucket)
		if b == nil {
			return nil
		}
		raw := b.Get([]byte(ip))
		if raw == nil {
			return nil
		}
		var e entry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil
		}
		found = &e
		return nil
	})
	if found == nil || found.Result == nil {
		return nil
	}
	if time.Since(found.Created) > c.ttl {
		// Left in place rather than deleted: expiry is read-mostly, and taking
		// a write transaction on every stale read would serialize a bulk run
		// behind the one lock bbolt has. Sweep handles removal.
		return nil
	}
	return found.Result
}

// Put stores an answer.
//
// Errors are swallowed by design. A full disk or a read-only home directory
// should not fail a lookup that already succeeded.
func (c *Cache) Put(ip string, result *vpndetection.Result) {
	if c == nil || result == nil {
		return
	}
	raw, err := json.Marshal(entry{Created: time.Now(), Result: result})
	if err != nil {
		return
	}
	_ = c.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(c.bucket)
		if b == nil {
			return nil
		}
		return b.Put([]byte(ip), raw)
	})
}

// PutBatch stores many answers in one transaction.
//
// bbolt starts a new mmap-backed write transaction per Update, so a bulk run
// doing one per address is dominated by transaction overhead rather than by the
// writes.
func (c *Cache) PutBatch(results map[string]*vpndetection.Result) {
	if c == nil || len(results) == 0 {
		return
	}
	now := time.Now()
	_ = c.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(c.bucket)
		if b == nil {
			return nil
		}
		for ip, result := range results {
			if result == nil {
				continue
			}
			raw, err := json.Marshal(entry{Created: now, Result: result})
			if err != nil {
				continue
			}
			if err := b.Put([]byte(ip), raw); err != nil {
				return err
			}
		}
		return nil
	})
}

// CacheStats is what `cache info` reports.
type CacheStats struct {
	Path    string
	Size    int64
	Buckets int
	Entries int
	Expired int
}

// InspectCache reports on the whole cache, across every credential.
func InspectCache(ttl time.Duration) (*CacheStats, error) {
	path, err := CachePath()
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return &CacheStats{Path: path}, nil
	}
	if err != nil {
		return nil, err
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 2 * time.Second, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer db.Close()

	stats := &CacheStats{Path: path, Size: st.Size()}
	err = db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(_ []byte, b *bbolt.Bucket) error {
			stats.Buckets++
			return b.ForEach(func(_, v []byte) error {
				stats.Entries++
				var e entry
				if err := json.Unmarshal(v, &e); err == nil && time.Since(e.Created) > ttl {
					stats.Expired++
				}
				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// ClearCache deletes the whole cache, every credential's entries with it.
func ClearCache() error {
	path, err := CachePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("could not clear the cache: %w", err)
	}
	return nil
}
