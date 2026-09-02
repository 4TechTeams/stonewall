package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// maxInclude caps a downloaded policy. A policy is a few hundred lines; anything larger is not one.
const maxInclude = 1 << 20

// staleAfter is how long a cached remote policy is used before Stonewall offers to refresh it.
const staleAfter = 7 * 24 * time.Hour

// lock records what every remote include resolved to, so a launch uses reviewed content or none.
type lock struct {
	Policies     map[string]lockEntry `yaml:"policies"`
	SnoozedUntil time.Time            `yaml:"snoozed_until,omitempty"`
}

type lockEntry struct {
	SHA256  string    `yaml:"sha256"`
	Fetched time.Time `yaml:"fetched"`
}

// UpdateResult reports what happened to one remote include during Update.
type UpdateResult struct {
	URL    string
	Status string // "updated" | "up to date" | "kept" | "failed"
	Err    error
}

// Update refreshes every remote include of the policy at path: unchanged content refreshes the fetch time,
// changed content is shown as a diff against the cached version and asked about, rejected changes keep the old
// version. It clears any snooze. One result per URL; err is non-nil when any URL failed.
func (l Loader) Update(path string) ([]UpdateResult, error) {
	local, err := Load(path)
	if err != nil {
		return nil, err
	}
	r, err := l.resolver(path)
	if err != nil {
		return nil, err
	}
	results := r.update(local.Include)
	if err := r.save(); err != nil {
		return results, err
	}
	failed := 0
	for _, res := range results {
		if res.Status == "failed" {
			failed++
		}
	}
	if failed > 0 {
		return results, fmt.Errorf("%d of %d remote policies could not be updated", failed, len(results))
	}
	return results, nil
}

// resolve carries the state of one Load or Update: the lock file, whether it changed, and the local files
// the caller must make read-only.
type resolve struct {
	l        Loader
	path     string // the policy file being resolved
	lockPath string
	lock     lock
	dirty    bool
	files    []string
}

func (l Loader) resolver(path string) (*resolve, error) {
	r := &resolve{l: l, path: path, lockPath: filepath.Join(filepath.Dir(path), ".stonewall", "policies", "lock.yml")}
	var err error
	if r.lock, err = readLock(r.lockPath); err != nil {
		return nil, fmt.Errorf("%s: %w", r.lockPath, err)
	}
	return r, nil
}

func (r *resolve) now() time.Time {
	if r.l.Now != nil {
		return r.l.Now()
	}
	return time.Now()
}

// ask is false when no Ask function is wired: without a user to review it, nothing is trusted.
func (r *resolve) ask(title, body, question string) bool {
	return r.l.Ask != nil && r.l.Ask(title, body, question)
}

func (r *resolve) cacheDir() string {
	return filepath.Join(filepath.Dir(r.path), ".stonewall", "policies", "cache")
}

func (r *resolve) cacheFile(u, hash string) string {
	return filepath.Join(r.cacheDir(), cacheName(u)+"-"+hash+".yml")
}

// cacheName is the URL's file name without extension, e.g. "claude" for .../policy/claude.yml.
func cacheName(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "policy"
	}
	name := strings.TrimSuffix(path.Base(u.Path), path.Ext(u.Path))
	if name == "" || name == "." || name == ".." || name == "/" {
		return "policy"
	}
	return name
}

// remoteURL validates an include that names a scheme. Content is pinned in lock.yml, never in the URL.
func remoteURL(inc string) (string, error) {
	if !strings.HasPrefix(inc, "https://") {
		return "", fmt.Errorf("include %s: only https:// URLs are supported", inc)
	}
	if strings.Contains(inc, "#") {
		return "", fmt.Errorf("include %s: pins are recorded in .stonewall/policies/lock.yml, not in the URL", inc)
	}
	return inc, nil
}

func remoteURLs(list []string) []string {
	var out []string
	for _, inc := range list {
		if !strings.Contains(inc, "://") {
			continue
		}
		if u, err := remoteURL(inc); err == nil {
			out = append(out, u)
		}
	}
	return out
}

// remote returns the policy for an https include: from the cache when the lock vouches for it, otherwise
// downloaded, shown and confirmed. Anything unverified aborts the launch.
func (r *resolve) remote(inc string) (Policy, error) {
	u, err := remoteURL(inc)
	if err != nil {
		return Policy{}, err
	}
	entry, locked := r.lock.Policies[u]
	if locked {
		cached := r.cacheFile(u, entry.SHA256)
		b, err := os.ReadFile(cached)
		switch {
		case err == nil:
			if digest(b) != entry.SHA256 {
				return Policy{}, fmt.Errorf("include %s: cache file %s does not match lock.yml (tampered or corrupt); run 'stonewall policy update'", inc, cached)
			}
			r.files = append(r.files, cached)
			return parseInclude(inc, b)
		case !errors.Is(err, fs.ErrNotExist):
			return Policy{}, fmt.Errorf("include %s: %w", inc, err)
		}
	}
	body, hash, err := r.l.download(u)
	if err != nil {
		return Policy{}, fmt.Errorf("include %s: %w", inc, err)
	}
	switch {
	case locked && hash == entry.SHA256: // the cache file was lost; this content is already trusted
	case locked:
		if !r.ask(changeTitle(u, entry.SHA256, hash), r.diff(u, entry.SHA256, body), "Accept the new version?") {
			return Policy{}, fmt.Errorf("include %s: rejected", inc)
		}
	default:
		if !r.ask(fmt.Sprintf("include %s (sha256 %s)", u, hash), string(body), "Trust this policy?") {
			return Policy{}, fmt.Errorf("include %s: rejected", inc)
		}
	}
	cached, err := r.record(u, hash, body)
	if err != nil {
		return Policy{}, fmt.Errorf("include %s: %w", inc, err)
	}
	r.files = append(r.files, cached)
	return parseInclude(inc, body)
}

// update refreshes every https include in order, leaving the lock dirty for the caller to save.
func (r *resolve) update(includes []string) []UpdateResult {
	var out []UpdateResult
	for _, inc := range includes {
		if !strings.Contains(inc, "://") {
			continue
		}
		u, err := remoteURL(inc)
		if err != nil {
			out = append(out, UpdateResult{URL: inc, Status: "failed", Err: err})
			continue
		}
		body, hash, err := r.l.download(u)
		if err != nil {
			out = append(out, UpdateResult{URL: u, Status: "failed", Err: err})
			continue
		}
		entry, locked := r.lock.Policies[u]
		status := "updated"
		switch {
		case locked && entry.SHA256 == hash:
			status = "up to date"
		case locked:
			if !r.ask(changeTitle(u, entry.SHA256, hash), r.diff(u, entry.SHA256, body), "Accept the new version?") {
				out = append(out, UpdateResult{URL: u, Status: "kept"})
				continue
			}
		default:
			if !r.ask(fmt.Sprintf("include %s (sha256 %s)", u, hash), string(body), "Trust this policy?") {
				out = append(out, UpdateResult{URL: u, Status: "kept"})
				continue
			}
		}
		if _, err := r.record(u, hash, body); err != nil {
			out = append(out, UpdateResult{URL: u, Status: "failed", Err: err})
			continue
		}
		out = append(out, UpdateResult{URL: u, Status: status})
	}
	r.lock.SnoozedUntil = time.Time{}
	r.dirty = true
	return out
}

func changeTitle(u, old, new string) string {
	return fmt.Sprintf("include %s changed (sha256 %s → %s)", u, old, new)
}

// diff shows what a new body changes against the cached version, or the whole body when there is nothing
// to compare against.
func (r *resolve) diff(u, oldHash string, body []byte) string {
	old := r.cacheFile(u, oldHash)
	if _, err := os.Stat(old); err != nil {
		return string(body)
	}
	bin, err := exec.LookPath("diff")
	if err != nil {
		return string(body)
	}
	tmp, err := os.CreateTemp("", "stonewall-policy-*.yml")
	if err != nil {
		return string(body)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return string(body)
	}
	tmp.Close()
	out, _ := exec.Command(bin, "-u", old, tmp.Name()).Output() // diff exits 1 when the files differ
	if len(out) == 0 {
		return string(body)
	}
	return string(out)
}

// record caches body, drops the cache file it replaces and points the lock entry at it.
func (r *resolve) record(u, hash string, body []byte) (string, error) {
	if old, ok := r.lock.Policies[u]; ok && old.SHA256 != hash {
		os.Remove(r.cacheFile(u, old.SHA256))
	}
	cached := r.cacheFile(u, hash)
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		return "", err
	}
	if err := replace(cached, body); err != nil {
		return "", err
	}
	if r.lock.Policies == nil {
		r.lock.Policies = map[string]lockEntry{}
	}
	r.lock.Policies[u] = lockEntry{SHA256: hash, Fetched: r.now()}
	r.dirty = true
	return cached, nil
}

// stale returns the age in whole days of the oldest cached entry among urls and whether it is past staleAfter.
func (r *resolve) stale(urls []string) (int, bool) {
	var oldest time.Duration
	now := r.now()
	for _, u := range urls {
		e, ok := r.lock.Policies[u]
		if !ok || e.Fetched.IsZero() {
			continue
		}
		if age := now.Sub(e.Fetched); age > oldest {
			oldest = age
		}
	}
	return int(oldest.Hours() / 24), oldest > staleAfter
}

func (r *resolve) snoozed() bool {
	return r.lock.SnoozedUntil.After(r.now())
}

func (r *resolve) fetchedList(urls []string) string {
	var b strings.Builder
	for _, u := range urls {
		if e, ok := r.lock.Policies[u]; ok {
			fmt.Fprintf(&b, "%s  fetched %s\n", u, e.Fetched.Format("2006-01-02"))
		}
	}
	return b.String()
}

func (r *resolve) save() error {
	if !r.dirty || len(r.lock.Policies) == 0 { // a lock exists only while remote policies do
		return nil
	}
	b, err := yaml.Marshal(r.lock)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.lockPath), 0o755); err != nil {
		return err
	}
	return replace(r.lockPath, b)
}

func readLock(path string) (lock, error) {
	l := lock{Policies: map[string]lockEntry{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return lock{}, err
	}
	if err := yaml.Unmarshal(b, &l); err != nil {
		return lock{}, err
	}
	if l.Policies == nil {
		l.Policies = map[string]lockEntry{}
	}
	return l, nil
}

// httpsOnly refuses a redirect that leaves https, so a policy is never fetched in the clear.
func httpsOnly(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("redirect to %s: only https:// is allowed", req.URL)
	}
	if len(via) >= 10 { // the limit http.Client applies when CheckRedirect is nil
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

// download fetches u and returns the body with its sha256, refusing anything but a 200 under 1 MiB
// that parses as a policy without includes of its own. Nothing is cached before this passes.
func (l Loader) download(u string) ([]byte, string, error) {
	client := l.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	c := *client // a copy: the caller's client keeps its own redirect policy
	c.CheckRedirect = httpsOnly
	resp, err := c.Get(u)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.New(resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxInclude+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) > maxInclude {
		return nil, "", errors.New("larger than 1 MiB")
	}
	p, err := Parse(body)
	if err != nil {
		return nil, "", err
	}
	if len(p.Include) > 0 {
		return nil, "", errors.New("nested includes are not supported")
	}
	return body, digest(body), nil
}

func digest(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
