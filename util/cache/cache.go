// Package cache keeps JSON copies of slow reads - the spreadsheet, the channel's
// playlists - in files that are reused for a while before being read again.
package cache

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// TTL is how long a cached copy of the spreadsheet or of the channel's playlists
// is used before it is read again. Long enough that a session of get and sync runs
// does not pay for the read each time, short enough that an edit made an hour ago is
// seen. --refresh reads again regardless.
const TTL = time.Hour

// Read fills v from a cache file, if there is one younger than TTL and refresh is
// false. Age is judged by the file's modification time. Returns the file's age and
// whether v was filled.
//
// An unreadable or corrupt cache is not an error: it is logged and treated as absent,
// and the caller reads from the source instead.
func Read(path string, refresh bool, v any) (time.Duration, bool) {
	if refresh {
		log.Printf("Not using the cache %s: a refresh was asked for", path)
		return 0, false
	}

	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	age := time.Since(info.ModTime())
	if age >= TTL {
		log.Printf("Not using the cache %s: it is %s old", path, age.Round(time.Minute))
		return age, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Not using the cache %s: %s", path, err)
		return age, false
	}
	if err := json.Unmarshal(data, v); err != nil {
		log.Printf("Not using the cache %s: %s", path, err)
		return age, false
	}

	return age, true
}

// Write saves v as a cache file. It is written to a temporary file and then
// renamed, so a run interrupted mid-write leaves the old cache or none, never half of
// one.
func Write(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", filepath.Dir(path), err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("cannot write the cache %s: %w", path, err)
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("cannot write the cache %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cannot write the cache %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("cannot write the cache %s: %w", path, err)
	}
	return nil
}
