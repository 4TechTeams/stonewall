package policy

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Loader resolves a policy file and its includes into one effective policy.
type Loader struct {
	// Ask shows body under title and asks question; false means no. It is also used for the staleness prompt.
	Ask    func(title, body, question string) bool
	Client *http.Client                 // https includes; tests inject an httptest client
	Now    func() time.Time             // defaults to time.Now; tests set it
	Report func(results []UpdateResult) // optional; receives what a staleness refresh did
}

// Load parses path, resolves each include in order and merges them with the local file applied last. It returns
// the effective policy and the absolute paths of every local file involved (local includes, cache files, the lock
// file and the cache directory); the caller makes those inside the project read-only. Remote includes are served
// from the cache; a cache miss downloads, shows and asks before caching. Stale caches trigger the update prompt.
func (l Loader) Load(path string) (Policy, []string, error) {
	local, err := Load(path)
	if err != nil {
		return Policy{}, nil, err
	}
	r, err := l.resolver(path)
	if err != nil {
		return Policy{}, nil, err
	}
	sources, err := r.includes(local.Include)
	if err != nil {
		return Policy{}, nil, err
	}
	if urls := remoteURLs(local.Include); len(urls) > 0 {
		if days, stale := r.stale(urls); stale && !r.snoozed() {
			title := fmt.Sprintf("Cached remote policies are older than %d days and might be outdated.", days)
			if r.ask(title, r.fetchedList(urls), "Update them now?") {
				results := r.update(local.Include) // a failed refresh leaves the trusted cached versions in place
				if l.Report != nil {
					l.Report(results)
				}
				if sources, err = r.includes(local.Include); err != nil {
					return Policy{}, nil, err
				}
			} else {
				r.lock.SnoozedUntil = r.now().Add(24 * time.Hour)
				r.dirty = true
			}
		}
		r.files = append(r.files, r.cacheDir(), r.lockPath)
		if dir := filepath.Dir(r.lockPath); exists(dir) { // the directory itself, so it cannot be renamed away
			r.files = append(r.files, dir)
		}
	}
	if err := r.save(); err != nil {
		return Policy{}, nil, err
	}
	eff, err := Merge(append(sources, local)...)
	if err != nil {
		return Policy{}, nil, err
	}
	return eff, absAll(r.files), nil
}

// includes resolves the include list in order, refusing anything it cannot read. It records the local
// files it used, so a second pass after an update starts from a clean list.
func (r *resolve) includes(list []string) ([]Policy, error) {
	r.files = nil
	sources := make([]Policy, 0, len(list))
	for _, inc := range list {
		var p Policy
		var err error
		if strings.Contains(inc, "://") {
			p, err = r.remote(inc)
		} else {
			var abs string
			p, abs, err = readLocalInclude(r.path, inc)
			if err == nil {
				r.files = append(r.files, abs)
			}
		}
		if err != nil {
			return nil, err
		}
		if len(p.Include) > 0 {
			return nil, fmt.Errorf("include %s: nested includes are not supported", inc)
		}
		sources = append(sources, p)
	}
	return sources, nil
}

// readLocalInclude resolves inc against the directory of path (or $HOME for ~/) and parses it.
// A relative entry must stay inside the tree of path; ~/ and absolute entries may live anywhere.
func readLocalInclude(path, inc string) (Policy, string, error) {
	abs, relative := inc, false
	switch {
	case strings.HasPrefix(inc, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return Policy{}, "", err
		}
		abs = filepath.Join(home, inc[2:])
	case !filepath.IsAbs(inc):
		abs = filepath.Join(filepath.Dir(path), inc)
		relative = true
	}
	abs, err := filepath.Abs(abs)
	if err != nil {
		return Policy{}, "", err
	}
	if relative {
		if err := insideTree(filepath.Dir(path), abs, inc); err != nil {
			return Policy{}, abs, err
		}
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return Policy{}, abs, fmt.Errorf("include %s: not found", inc)
	}
	p, err := parseInclude(inc, b)
	return p, abs, err
}

// insideTree refuses abs when it resolves outside dir: a ../ entry, or a symlink pointing out of the
// project. A path that cannot be resolved does not exist; the read reports that instead.
func insideTree(dir, abs, inc string) error {
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil
	}
	root, err := filepath.EvalSymlinks(dir)
	if err == nil && (real == root || strings.HasPrefix(real, root+string(filepath.Separator))) {
		return nil
	}
	return fmt.Errorf("include %s: resolves to %s, outside the project", inc, real)
}

func parseInclude(inc string, b []byte) (Policy, error) {
	p, err := Parse(b)
	if err != nil {
		return Policy{}, fmt.Errorf("include %s: %w", inc, err)
	}
	return p, nil
}

func absAll(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if a, err := filepath.Abs(p); err == nil {
			p = a
		}
		out = append(out, p)
	}
	return out
}

// replace writes content over path through a temp file in the same directory, so a crash leaves the old file.
func replace(path string, content []byte) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".stonewall-tmp-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(content); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(f.Name(), mode); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
