package policy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var epoch = time.Date(2026, 9, 2, 15, 4, 5, 0, time.UTC)

// policyServer serves a body the test can change and counts the requests, so a test can prove that a
// cached policy is not fetched again.
type policyServer struct {
	*httptest.Server
	body string
	hits int
}

func newPolicyServer(t *testing.T, body string) *policyServer {
	s := &policyServer{body: body}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits++
		io.WriteString(w, s.body)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *policyServer) url() string { return s.URL + "/claude.yml" }

type asker struct {
	titles []string
	answer bool
}

func (a *asker) ask(title, body, question string) bool {
	a.titles = append(a.titles, title)
	return a.answer
}

// fixture writes a policy file including srv and returns its path plus a clock the test can move.
func fixture(t *testing.T, srv *policyServer) (string, *time.Time) {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("include:\n  - "+srv.url()+"\nbin:\n  allowed: [sh]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := epoch
	return path, &now
}

func loader(srv *policyServer, a *asker, now *time.Time) Loader {
	return Loader{Client: srv.Client(), Ask: a.ask, Now: func() time.Time { return *now }}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRemoteIncludeCached(t *testing.T) {
	body := "bin:\n  allowed: [git]\n"
	srv := newPolicyServer(t, body)
	path, now := fixture(t, srv)
	a := &asker{answer: true}
	l := loader(srv, a, now)

	dir := filepath.Dir(path)
	cache := filepath.Join(dir, ".stonewall", "policies", "cache", "claude-"+digest([]byte(body))+".yml")
	lockFile := filepath.Join(dir, ".stonewall", "policies", "lock.yml")

	// 1. First load: reviewed once, cached, locked.
	eff, files, err := l.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.titles) != 1 || !strings.Contains(a.titles[0], srv.url()) {
		t.Fatalf("Ask titles: %v", a.titles)
	}
	if got := readFile(t, cache); got != body {
		t.Errorf("cache file: %q", got)
	}
	lk, err := readLock(lockFile)
	if err != nil {
		t.Fatal(err)
	}
	entry := lk.Policies[srv.url()]
	if entry.SHA256 != digest([]byte(body)) || !entry.Fetched.Equal(epoch) {
		t.Errorf("lock entry: %+v", entry)
	}
	if want := []string{"git", "sh"}; !reflect.DeepEqual(eff.Bin.Allowed, want) {
		t.Errorf("effective bin.allowed: got %v want %v", eff.Bin.Allowed, want)
	}
	for _, want := range []string{cache, lockFile} {
		if !contains(files, want) {
			t.Errorf("read-only files %v missing %s", files, want)
		}
	}
	if srv.hits != 1 {
		t.Errorf("server hits %d, want 1", srv.hits)
	}

	// 2. Second load: served from the cache, no prompt and no request.
	a.titles = nil
	eff2, _, err := l.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.titles) != 0 || srv.hits != 1 {
		t.Errorf("cached load asked %v and hit the server %d times", a.titles, srv.hits)
	}
	if !reflect.DeepEqual(eff, eff2) {
		t.Errorf("cached load differs: %+v vs %+v", eff, eff2)
	}

	// 8. Cache file gone but the lock still vouches for the content: restore it without asking.
	if err := os.Remove(cache); err != nil {
		t.Fatal(err)
	}
	a.titles = nil
	if _, _, err := l.Load(path); err != nil {
		t.Fatal(err)
	}
	if len(a.titles) != 0 || srv.hits != 2 {
		t.Errorf("restore asked %v, hits %d (want 0 asks, 2 hits)", a.titles, srv.hits)
	}
	if got := readFile(t, cache); got != body {
		t.Errorf("restored cache file: %q", got)
	}

	// 3. A cache file that no longer matches the lock is never used.
	f, err := os.OpenFile(cache, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("# tampered\n")
	f.Close()
	if _, _, err := l.Load(path); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("tampered cache accepted: %v", err)
	}
}

func TestRemoteIncludeRejected(t *testing.T) {
	srv := newPolicyServer(t, "bin:\n  allowed: [git]\n")
	path, now := fixture(t, srv)
	a := &asker{answer: false}
	if _, _, err := loader(srv, a, now).Load(path); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("unreviewed policy accepted: %v", err)
	}
}

func TestRemoteIncludeErrors(t *testing.T) {
	srv := newPolicyServer(t, "include: [other.yml]\nbin:\n  allowed: [git]\n")
	a := &asker{answer: true}
	load := func(t *testing.T, inc string) error {
		path := filepath.Join(t.TempDir(), FileName)
		if err := os.WriteFile(path, []byte("include:\n  - "+inc+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		now := epoch
		_, _, err := loader(srv, a, &now).Load(path)
		return err
	}
	if err := load(t, srv.url()+"#sha256=abc"); err == nil || !strings.Contains(err.Error(), "lock.yml") {
		t.Errorf("pin in URL: %v", err)
	}
	if err := load(t, "http://example.invalid/p.yml"); err == nil || !strings.Contains(err.Error(), "only https") {
		t.Errorf("http include: %v", err)
	}
	if err := load(t, srv.url()); err == nil || !strings.Contains(err.Error(), "nested") {
		t.Errorf("nested remote include: %v", err)
	}
}

// A body that is not a flat policy is refused before the user is asked and before anything is written.
func TestRemoteIncludeInvalidLeavesNoTrace(t *testing.T) {
	srv := newPolicyServer(t, "include: [other.yml]\nbin:\n  allowed: [git]\n")
	path, now := fixture(t, srv)
	a := &asker{answer: true}
	if _, _, err := loader(srv, a, now).Load(path); err == nil || !strings.Contains(err.Error(), "nested") {
		t.Fatalf("nested remote include: %v", err)
	}
	if len(a.titles) != 0 {
		t.Errorf("asked to trust a body that was then refused: %v", a.titles)
	}
	if entries, err := os.ReadDir(filepath.Join(filepath.Dir(path), ".stonewall", "policies")); err == nil && len(entries) > 0 {
		t.Errorf("rejected body left %d entries behind", len(entries))
	}
}

// A redirect out of https is refused: a policy is never fetched in the clear.
func TestDownloadRefusesInsecureRedirect(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.invalid/p.yml", http.StatusFound)
	}))
	defer srv.Close()
	_, _, err := Loader{Client: srv.Client()}.download(srv.URL + "/claude.yml")
	if err == nil || !strings.Contains(err.Error(), "only https:// is allowed") {
		t.Fatalf("insecure redirect: %v", err)
	}
}

func TestRemoteIncludeStale(t *testing.T) {
	body := "bin:\n  allowed: [git]\n"
	srv := newPolicyServer(t, body)
	path, now := fixture(t, srv)
	a := &asker{answer: true}
	l := loader(srv, a, now)
	if _, _, err := l.Load(path); err != nil {
		t.Fatal(err)
	}
	lockFile := filepath.Join(filepath.Dir(path), ".stonewall", "policies", "lock.yml")

	// 5. Older than a week and the answer is no: snoozed for a day, still loaded from the cache.
	*now = epoch.Add(8 * 24 * time.Hour)
	a.answer = false
	a.titles = nil
	eff, _, err := l.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.titles) != 1 || !strings.Contains(a.titles[0], "8 days") {
		t.Fatalf("staleness prompt: %v", a.titles)
	}
	if want := []string{"git", "sh"}; !reflect.DeepEqual(eff.Bin.Allowed, want) {
		t.Errorf("stale load: %v", eff.Bin.Allowed)
	}
	lk, err := readLock(lockFile)
	if err != nil {
		t.Fatal(err)
	}
	if !lk.SnoozedUntil.Equal(now.Add(24 * time.Hour)) {
		t.Errorf("SnoozedUntil %v, want %v", lk.SnoozedUntil, now.Add(24*time.Hour))
	}
	if srv.hits != 1 {
		t.Errorf("declining the update fetched anyway: %d hits", srv.hits)
	}

	// The snooze holds: no second prompt within the day.
	a.titles = nil
	if _, _, err := l.Load(path); err != nil {
		t.Fatal(err)
	}
	if len(a.titles) != 0 {
		t.Errorf("asked again while snoozed: %v", a.titles)
	}

	// 6. Accepting the offer refreshes the fetch time; unchanged content needs no second question.
	*now = epoch.Add(30 * 24 * time.Hour)
	a.answer = true
	a.titles = nil
	if _, _, err := l.Load(path); err != nil {
		t.Fatal(err)
	}
	if len(a.titles) != 1 {
		t.Errorf("unchanged refresh asked %v", a.titles)
	}
	lk, err = readLock(lockFile)
	if err != nil {
		t.Fatal(err)
	}
	if !lk.Policies[srv.url()].Fetched.Equal(*now) || !lk.SnoozedUntil.IsZero() {
		t.Errorf("after refresh: %+v", lk)
	}
}

func TestUpdate(t *testing.T) {
	body := "bin:\n  allowed: [git]\n"
	srv := newPolicyServer(t, body)
	path, now := fixture(t, srv)
	a := &asker{answer: true}
	l := loader(srv, a, now)
	if _, _, err := l.Load(path); err != nil {
		t.Fatal(err)
	}
	cacheOf := func(b string) string {
		return filepath.Join(filepath.Dir(path), ".stonewall", "policies", "cache", "claude-"+digest([]byte(b))+".yml")
	}
	changed := "bin:\n  allowed: [git, make]\n"
	srv.body = changed

	// 7a. A refused change keeps the reviewed version.
	a.answer = false
	a.titles = nil
	res, err := l.Update(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Status != "kept" {
		t.Fatalf("refused update: %+v", res)
	}
	if readFile(t, cacheOf(body)) != body {
		t.Error("refused update touched the cache")
	}

	// 7b. An accepted change replaces the cache file and the lock entry.
	a.answer = true
	a.titles = nil
	res, err = l.Update(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Status != "updated" {
		t.Fatalf("accepted update: %+v", res)
	}
	if len(a.titles) != 1 || !strings.Contains(a.titles[0], "changed") {
		t.Errorf("change prompt: %v", a.titles)
	}
	if readFile(t, cacheOf(changed)) != changed {
		t.Error("new cache file missing")
	}
	if _, err := os.Stat(cacheOf(body)); !os.IsNotExist(err) {
		t.Error("old cache file kept")
	}
	lk, err := readLock(filepath.Join(filepath.Dir(path), ".stonewall", "policies", "lock.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if lk.Policies[srv.url()].SHA256 != digest([]byte(changed)) {
		t.Errorf("lock not updated: %+v", lk)
	}

	// 7c. Unchanged content only refreshes the fetch time.
	*now = epoch.Add(time.Hour)
	res, err = l.Update(path)
	if err != nil || len(res) != 1 || res[0].Status != "up to date" {
		t.Fatalf("unchanged update: %+v %v", res, err)
	}

	// 7d. An unreachable policy is a failure, never a silent skip.
	srv.Close()
	res, err = l.Update(path)
	if err == nil || len(res) != 1 || res[0].Status != "failed" || res[0].Err == nil {
		t.Fatalf("unreachable update: %+v %v", res, err)
	}
}

func TestLoadLocalInclude(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "policies"), 0o755); err != nil {
		t.Fatal(err)
	}
	inc := filepath.Join(dir, "policies", "base.yml")
	if err := os.WriteFile(inc, []byte("bin:\n  allowed: [git, curl]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("include:\n  - policies/base.yml\nbin:\n  allowed: [sh]\n  denied: [curl]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The include is applied first, the local file last: it keeps git and takes curl away again.
	eff, local, err := Loader{}.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"git", "sh"}; !reflect.DeepEqual(eff.Bin.Allowed, want) {
		t.Errorf("effective bin.allowed: got %v want %v", eff.Bin.Allowed, want)
	}
	if len(eff.Include) != 0 {
		t.Errorf("effective policy still carries includes: %v", eff.Include)
	}
	if want := []string{inc}; !reflect.DeepEqual(local, want) {
		t.Errorf("local includes: got %v want %v", local, want)
	}

	// An absolute include resolves too, and is reported so the caller can make it read-only.
	if err := os.WriteFile(path, []byte("include:\n  - "+inc+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, local, err := (Loader{}).Load(path); err != nil || !reflect.DeepEqual(local, []string{inc}) {
		t.Errorf("absolute include: %v %v", local, err)
	}

	// A nested include is refused rather than silently ignored.
	if err := os.WriteFile(inc, []byte("include:\n  - other.yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (Loader{}).Load(path); err == nil || !strings.Contains(err.Error(), "nested") {
		t.Errorf("nested local include: %v", err)
	}
}

func TestLocalIncludeOutsideProject(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "proj")
	if err := os.MkdirAll(filepath.Join(proj, "policies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "outside"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "outside", "x.yml")
	if err := os.WriteFile(out, []byte("bin:\n  allowed: [git]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "outside", "x.yml"), filepath.Join(proj, "policies", "x.yml")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(proj, FileName)
	load := func(inc string) error {
		if err := os.WriteFile(path, []byte("include:\n  - "+inc+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, err := (Loader{}).Load(path)
		return err
	}
	// A relative entry may not leave the project, through ../ or through a symlink.
	for _, inc := range []string{"../outside/x.yml", "policies/x.yml"} {
		if err := load(inc); err == nil || !strings.Contains(err.Error(), "outside the project") {
			t.Errorf("include %s: %v", inc, err)
		}
	}
	// The same file named absolutely is the user's explicit choice and loads.
	if err := load(out); err != nil {
		t.Errorf("absolute include outside the project: %v", err)
	}
}

func TestLoadMissingInclude(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("include:\n  - policies/missing.yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (Loader{}).Load(path); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing local include: %v", err)
	}
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}

func TestNoLockWithoutRemotes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "policies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policies", "base.yml"), []byte("bin:\n  allowed: [git]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("include:\n  - policies/base.yml\nbin:\n  allowed: [sh]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (Loader{}).Load(path); err != nil {
		t.Fatal(err)
	}
	if res, err := (Loader{}).Update(path); err != nil || len(res) != 0 {
		t.Fatalf("update without remotes: %v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".stonewall")); !os.IsNotExist(err) {
		t.Fatalf(".stonewall created without remote policies (err=%v)", err)
	}
}
