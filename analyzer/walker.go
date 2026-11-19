package analyzer

import (
	"context"
	"fmt"

	"github.com/holacloud/clocme/analyzer/linguist"
	"github.com/holacloud/clocme/model"
)

const gitattributesPath = ".gitattributes"

// fileInfo is the state of one regular file of the current tree.
type fileInfo struct {
	blob    string
	lang    string      // "" when not counted
	stats   model.Stats // contribution to totals (Files=1 when counted)
	counted bool
}

// walker holds the tree state while walking the history.
type walker struct {
	cat     *catFile
	cache   *blobCache
	maxBlob int64

	files  map[string]*fileInfo
	attrs  *linguist.Attributes
	totals map[string]model.Stats
}

// evaluate computes the contribution of a file, counting the blob if needed.
func (w *walker) evaluate(p, blob string) (*fileInfo, error) {
	fi := &fileInfo{blob: blob}

	excluded, override := linguist.Classify(p, w.attrs)
	if excluded {
		return fi, nil
	}
	key, candidates := detectionKey(p, override)
	if key == "" {
		return fi, nil
	}

	ck := cacheKey(blob, key)
	e, ok := w.cache.Entries[ck]
	if !ok {
		size, err := w.cat.size(blob)
		if err != nil {
			return nil, err
		}
		if size > w.maxBlob {
			e = cacheEntry{Skip: true}
		} else {
			content, err := w.cat.contents(blob)
			if err != nil {
				return nil, err
			}
			lang, code, comments, blanks, counted := count(p, candidates, content)
			e = cacheEntry{Lang: lang, Code: code, Comments: comments, Blanks: blanks, Skip: !counted}
		}
		w.cache.Entries[ck] = e
		w.cache.dirty = true
	}
	if e.Skip {
		return fi, nil
	}

	fi.lang = e.Lang
	fi.counted = true
	fi.stats = model.Stats{Code: e.Code, Comments: e.Comments, Blanks: e.Blanks, Files: 1}
	return fi, nil
}

func (w *walker) addTotals(fi *fileInfo) {
	if !fi.counted {
		return
	}
	s := w.totals[fi.lang]
	s.Add(fi.stats)
	w.totals[fi.lang] = s
}

func (w *walker) subTotals(fi *fileInfo) {
	if !fi.counted {
		return
	}
	s := w.totals[fi.lang]
	s.Sub(fi.stats)
	if s.IsZero() {
		delete(w.totals, fi.lang)
	} else {
		w.totals[fi.lang] = s
	}
}

func (w *walker) set(p, blob string) error {
	if old, ok := w.files[p]; ok {
		if old.blob == blob {
			return nil
		}
		w.subTotals(old)
	}
	fi, err := w.evaluate(p, blob)
	if err != nil {
		return err
	}
	w.files[p] = fi
	w.addTotals(fi)
	return nil
}

func (w *walker) remove(p string) {
	if old, ok := w.files[p]; ok {
		w.subTotals(old)
		delete(w.files, p)
	}
}

// reevaluateAll recomputes every file and the totals; used when the
// attributes change (and to build the initial state from a tree).
func (w *walker) reevaluateAll() error {
	w.totals = map[string]model.Stats{}
	for p, old := range w.files {
		fi, err := w.evaluate(p, old.blob)
		if err != nil {
			return err
		}
		w.files[p] = fi
		w.addTotals(fi)
	}
	return nil
}

func (w *walker) loadAttrs(blob string) error {
	if blob == "" {
		w.attrs = nil
		return nil
	}
	size, err := w.cat.size(blob)
	if err != nil {
		return err
	}
	if size > w.maxBlob {
		w.attrs = nil
		return nil
	}
	content, err := w.cat.contents(blob)
	if err != nil {
		return err
	}
	w.attrs = linguist.ParseGitattributes(content)
	return nil
}

// loadTree rebuilds the state from the full tree of a commit.
func (w *walker) loadTree(ctx context.Context, dir, rev string) error {
	entries, err := lsTree(ctx, dir, rev)
	if err != nil {
		return err
	}
	w.files = map[string]*fileInfo{}
	attrsBlob := ""
	for _, e := range entries {
		if !isRegularFile(e.mode) {
			continue
		}
		if e.path == gitattributesPath {
			attrsBlob = e.blob
		}
		w.files[e.path] = &fileInfo{blob: e.blob}
	}
	if err := w.loadAttrs(attrsBlob); err != nil {
		return err
	}
	return w.reevaluateAll()
}

func (w *walker) snapshot() map[string]model.Stats {
	out := make(map[string]model.Stats, len(w.totals))
	for lang, s := range w.totals {
		if !s.IsZero() {
			out[lang] = s
		}
	}
	return out
}

// walk processes commits (in order) appending one point per commit.
func (w *walker) walk(ctx context.Context, dir string, commits []commit, points *[]model.Point, total int, progress func(done, total int)) error {
	ds, err := startDiffStream(ctx, dir, commits)
	if err != nil {
		return err
	}
	defer ds.close()

	var lastTime int64
	if n := len(*points); n > 0 {
		lastTime = (*points)[n-1].Time
	}

	for i, c := range commits {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := ds.next(c.sha)
		if err != nil {
			return err
		}

		// Attributes first: if they change, every file must be re-evaluated.
		attrsChanged := false
		for _, e := range entries {
			if e.path != gitattributesPath {
				continue
			}
			attrsChanged = true
			blob := ""
			if e.status != 'D' && isRegularFile(e.newMode) {
				blob = e.newBlob
			}
			if err := w.loadAttrs(blob); err != nil {
				return err
			}
		}

		for _, e := range entries {
			regular := e.status != 'D' && isRegularFile(e.newMode)
			switch {
			case !regular:
				w.remove(e.path)
			case attrsChanged:
				// just record the blob, reevaluateAll does the counting
				if old, ok := w.files[e.path]; ok {
					old.blob = e.newBlob
				} else {
					w.files[e.path] = &fileInfo{blob: e.newBlob}
				}
			default:
				if err := w.set(e.path, e.newBlob); err != nil {
					return fmt.Errorf("commit %s, file %s: %w", c.sha, e.path, err)
				}
			}
		}
		if attrsChanged {
			if err := w.reevaluateAll(); err != nil {
				return err
			}
		}

		t := c.time
		if t < lastTime {
			t = lastTime
		}
		lastTime = t
		*points = append(*points, model.Point{Commit: c.sha, Time: t, Languages: w.snapshot()})

		if progress != nil && (i+1)%200 == 0 {
			progress(len(*points), total)
		}
	}
	return ds.close()
}
