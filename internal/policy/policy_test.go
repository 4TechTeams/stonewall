package policy

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	p, err := Parse([]byte("include: [policies/base.yml, https://x/y.yml]\nproject:\n  readonly: [.git]\n  hidden: [.env]\n  writable: [docs]\nbin:\n  allowed: [make]\n  denied: [bash]\nexpose:\n  read: [~/.npm]\n  write: [~/.cache]\n  none: [~/.gem]\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := Policy{
		Include: []string{"policies/base.yml", "https://x/y.yml"},
		Bin:     Bin{Allowed: []string{"make"}, Denied: []string{"bash"}},
		Project: Project{Readonly: []string{".git"}, Hidden: []string{".env"}, Writable: []string{"docs"}},
		Expose:  Expose{Read: []string{"~/.npm"}, Write: []string{"~/.cache"}, None: []string{"~/.gem"}},
	}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("got %+v want %+v", p, want)
	}
	if _, err := Parse([]byte("project:\n  readonly: [.git]\nbogus: 1\n")); err == nil {
		t.Fatal("unknown top-level key accepted")
	}
	if _, err := Parse([]byte("bin:\n  allow: [x]\n")); err == nil {
		t.Fatal("unknown nested key accepted")
	}
	if _, err := Parse([]byte("expose:\n  read: [foo/bar]\n")); err == nil || !strings.Contains(err.Error(), "must start with") {
		t.Fatalf("bad expose entry: %v", err)
	}
	if _, err := Parse([]byte("expose:\n  none: [foo/bar]\n")); err == nil || !strings.Contains(err.Error(), "must start with") {
		t.Fatalf("bad expose none entry: %v", err)
	}
	if p, err := Parse([]byte("# only comments\n")); err != nil || !reflect.DeepEqual(p, Policy{}) {
		t.Fatalf("empty doc: %+v %v", p, err)
	}
}

func TestMerge(t *testing.T) {
	// (a) a later source reclassifies a project path and an expose path.
	got, err := Merge(
		Policy{Project: Project{Hidden: []string{".git"}}, Expose: Expose{Read: []string{"~/.npm"}}},
		Policy{Project: Project{Readonly: []string{".git"}}, Expose: Expose{Write: []string{"~/.npm"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := Policy{
		Project: Project{Readonly: []string{".git"}},
		Expose:  Expose{Write: []string{"~/.npm"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reclassify: got %+v want %+v", got, want)
	}

	// (b) the negative lists remove what an earlier source granted.
	got, err = Merge(
		Policy{
			Bin:     Bin{Allowed: []string{"sh", "curl"}},
			Project: Project{Hidden: []string{".env"}, Readonly: []string{".git"}},
			Expose:  Expose{Read: []string{"~/.npm"}, Write: []string{"~/.cache"}},
		},
		Policy{
			Bin:     Bin{Denied: []string{"curl"}},
			Project: Project{Writable: []string{".env"}},
			Expose:  Expose{None: []string{"~/.npm"}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	want = Policy{
		Bin:     Bin{Allowed: []string{"sh"}},
		Project: Project{Readonly: []string{".git"}},
		Expose:  Expose{Write: []string{"~/.cache"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("negatives: got %+v want %+v", got, want)
	}

	// (c) the local file is applied last and wins over both includes.
	got, err = Merge(
		Policy{Bin: Bin{Allowed: []string{"git"}}},
		Policy{Bin: Bin{Denied: []string{"git"}}},
		Policy{Bin: Bin{Allowed: []string{"git", "make"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Policy{Bin: Bin{Allowed: []string{"git", "make"}}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("local wins: got %+v want %+v", got, want)
	}

	// (d) the same item in two lists of one source is an error naming both lists.
	for _, c := range []struct {
		src  Policy
		want []string
	}{
		{Policy{Bin: Bin{Allowed: []string{"bash"}, Denied: []string{"bash"}}}, []string{"bin.allowed", "bin.denied"}},
		{Policy{Project: Project{Hidden: []string{"docs"}, Writable: []string{"docs/"}}}, []string{"project.hidden", "project.writable"}},
		{Policy{Expose: Expose{Read: []string{"~/.npm"}, None: []string{"~/.npm"}}}, []string{"expose.read", "expose.none"}},
	} {
		_, err := Merge(c.src)
		if err == nil {
			t.Fatalf("%+v: no error", c.src)
		}
		for _, w := range c.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("error %q does not name %s", err, w)
			}
		}
	}

	// (e) a trailing slash is not a different path: it evades neither a negation nor the conflict check.
	got, err = Merge(
		Policy{Expose: Expose{Read: []string{"/a/b/"}}},
		Policy{Expose: Expose{None: []string{"/a/b"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Expose.Read) != 0 {
		t.Errorf("trailing slash survived expose.none: %v", got.Expose.Read)
	}
	if _, err := Merge(Policy{Expose: Expose{Read: []string{"/a/b"}, None: []string{"/a/b/"}}}); err == nil {
		t.Error("trailing slash evaded the expose conflict check")
	}

	// (f) the result carries no Include and keeps first-seen order.
	got, err = Merge(
		Policy{Include: []string{"a.yml"}, Bin: Bin{Allowed: []string{"sh", "git"}}},
		Policy{Include: []string{"b.yml"}, Bin: Bin{Allowed: []string{"make", "git"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Policy{Bin: Bin{Allowed: []string{"sh", "git", "make"}}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("order: got %+v want %+v", got, want)
	}
}

func TestFindRoot(t *testing.T) {
	tmp := t.TempDir()
	mk := func(parts ...string) string {
		p := filepath.Join(append([]string{tmp}, parts...)...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// A nearer .git wins over a more distant policy file.
	mk("a", ".git")
	mk("a", "sub", ".git")
	deep := mk("a", "sub", "x")
	if err := os.WriteFile(filepath.Join(tmp, "a", FileName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindRoot(deep); got != filepath.Join(tmp, "a", "sub") {
		t.Fatalf("nearer .git: got %s", got)
	}
	// A policy file and .git in the same directory: that directory wins.
	mk("a2")
	if err := os.WriteFile(filepath.Join(tmp, "a2", FileName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	mk("a2", ".git")
	if got := FindRoot(mk("a2", "x", "y")); got != filepath.Join(tmp, "a2") {
		t.Fatalf("same dir: got %s", got)
	}
	// .git without a policy.
	mk("b", ".git")
	if got := FindRoot(mk("b", "src")); got != filepath.Join(tmp, "b") {
		t.Fatalf("git: got %s", got)
	}
	// Neither: the directory itself.
	plain := mk("c")
	if got := FindRoot(plain); got != plain {
		t.Fatalf("plain: got %s", got)
	}
	// No infinite loop at the filesystem root.
	if got := FindRoot("/"); got != "/" {
		t.Fatalf("root: got %s", got)
	}
}

func TestScaffold(t *testing.T) {
	claude := Scaffold("/opt/bin/claude")
	for _, want := range []string{"https://stonewall.sh/policy/base.yml", "https://stonewall.sh/policy/claude.yml"} {
		if !strings.Contains(claude, want) {
			t.Errorf("claude scaffold missing %s:\n%s", want, claude)
		}
	}
	sh := Scaffold("sh")
	if !strings.Contains(sh, "https://stonewall.sh/policy/base.yml") || strings.Contains(sh, "claude.yml") {
		t.Errorf("sh scaffold wrong:\n%s", sh)
	}
	p, err := Parse([]byte(claude))
	if err != nil {
		t.Fatal(err)
	}
	want := Policy{Include: []string{"https://stonewall.sh/policy/base.yml", "https://stonewall.sh/policy/claude.yml"}}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("parsed scaffold: got %+v want %+v", p, want)
	}

	path := filepath.Join(t.TempDir(), FileName)
	if err := WriteScaffold(path, claude); err != nil {
		t.Fatal(err)
	}
	if err := WriteScaffold(path, claude); err == nil {
		t.Fatal("overwrote existing policy")
	}
}
