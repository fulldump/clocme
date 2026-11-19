package analyzer

import (
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// cacheEntry is the result of counting one blob with one detection key.
type cacheEntry struct {
	Lang     string
	Code     int64
	Comments int64
	Blanks   int64
	Skip     bool // not counted (binary, minified, too big...)
}

// blobCache maps "<blob sha>|<detection key>" to its counts. Blobs are
// content addressed, so an entry never goes stale.
type blobCache struct {
	Entries map[string]cacheEntry
	dirty   bool
}

func cacheKey(blob, detection string) string { return blob + "|" + detection }

func loadCache(filename string) (*blobCache, error) {
	c := &blobCache{Entries: map[string]cacheEntry{}}
	f, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := gob.NewDecoder(f).Decode(c); err != nil {
		// A corrupt cache is not fatal: it is only an optimization.
		return &blobCache{Entries: map[string]cacheEntry{}}, nil
	}
	if c.Entries == nil {
		c.Entries = map[string]cacheEntry{}
	}
	return c, nil
}

func (c *blobCache) save(filename string) error {
	if !c.dirty {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := gob.NewEncoder(tmp).Encode(c); err != nil {
		tmp.Close()
		return fmt.Errorf("encoding cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), filename); err != nil {
		return err
	}
	c.dirty = false
	return nil
}
