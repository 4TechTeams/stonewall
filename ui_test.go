package main

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUIRows(t *testing.T) {
	rows := func(u ui) string {
		var buf bytes.Buffer
		u.w = &buf
		u.rows(
			[2]string{"project", "/Users/x/demo"},
			[2]string{"policy", ".stonewall.yml (created)"},
			[2]string{"sandbox", "sandbox-exec"},
		)
		u.warn("cowsay needs perl, which is not in bin.allowed")
		u.error(errors.New("boom"))
		return buf.String()
	}

	plain := rows(ui{plain: true})
	want := "stonewall: project /Users/x/demo\n" +
		"stonewall: policy .stonewall.yml (created)\n" +
		"stonewall: sandbox sandbox-exec\n" +
		"stonewall: warning cowsay needs perl, which is not in bin.allowed\n" +
		"stonewall: error boom\n"
	if plain != want {
		t.Errorf("plain output:\n%q\nwant:\n%q", plain, want)
	}

	colored := rows(ui{plain: false, color: true})
	if !strings.Contains(colored, "\x1b[1;38;5;202mstonewall\x1b[0m") {
		t.Errorf("colored output missing header escape: %q", colored)
	}
	if !strings.Contains(colored, "\x1b[2mproject \x1b[0m") {
		t.Errorf("colored output missing padded/dim project label: %q", colored)
	}
	if !strings.Contains(colored, "\x1b[2mpolicy  \x1b[0m") {
		t.Errorf("colored output missing padded/dim policy label: %q", colored)
	}
	if !strings.Contains(colored, "\x1b[1;31merror   \x1b[0m") {
		t.Errorf("colored output missing bold red error label: %q", colored)
	}
	if !strings.Contains(colored, "\x1b[1;33mwarning \x1b[0m") {
		t.Errorf("colored output missing bold yellow warning label: %q", colored)
	}

	formatted := rows(ui{plain: false, color: false})
	if strings.Contains(formatted, "\x1b") {
		t.Errorf("formatted-no-color output should have no escape codes: %q", formatted)
	}
}

func TestBlockAndConfirm(t *testing.T) {
	var buf bytes.Buffer
	u := ui{w: &buf, plain: true}
	u.block("stonewall will create .stonewall.yml", "include:\n  - https://stonewall.sh/policies/base.yml\n")
	want := "stonewall will create .stonewall.yml\n" +
		"  include:\n" +
		"    - https://stonewall.sh/policies/base.yml\n"
	if buf.String() != want {
		t.Errorf("block output:\n%q\nwant:\n%q", buf.String(), want)
	}

	for _, c := range []struct {
		in   string
		want bool
	}{{"y\n", true}, {"Y\n", true}, {"yes\n", true}, {"n\n", false}, {"\n", false}, {"", false}, {"maybe\n", false}} {
		stdin = strings.NewReader(c.in)
		buf.Reset()
		if got := u.confirm("Write it?"); got != c.want {
			t.Errorf("confirm(%q) = %v, want %v", c.in, got, c.want)
		}
		if buf.String() != "Write it? [y/N] " {
			t.Errorf("prompt: %q", buf.String())
		}
	}
	stdin = os.Stdin

	// A second question reads the second line, not what the first left buffered.
	stdin = strings.NewReader("y\nn\n")
	if !u.confirm("first") || u.confirm("second") {
		t.Error("consecutive confirms read the wrong lines")
	}
	stdin = os.Stdin
}

func TestNewUI(t *testing.T) {
	if newUI(&bytes.Buffer{}, false).plain {
		t.Error("newUI with a non-file writer should not be plain")
	}
	if !newUI(&bytes.Buffer{}, true).plain {
		t.Error("newUI with plain=true should be plain")
	}
}

func TestPick(t *testing.T) {
	var buf bytes.Buffer
	u := ui{w: &buf, plain: true}
	items := []pickItem{
		{Label: "Base", Detail: "Bare minimum safety net.", Checked: true},
		{Label: "Claude Code", Detail: "The minimum for Claude Code."},
	}
	defer func() { stdin = os.Stdin }()

	// down, tick Claude, down again (stays), up, untick Base, enter; "y\n" is left for the next prompt
	stdin = strings.NewReader("\x1b[B j\x1b[A \ry\n")
	got, ok := u.pick("official policies", "space toggles", items)
	if !ok || !reflect.DeepEqual(got, []bool{false, true}) {
		t.Fatalf("pick: got %v ok %v", got, ok)
	}
	for _, want := range []string{"official policies", "space toggles", "[x] Base", "[ ] Claude Code", "> [x] Claude Code", "[ ] Base", "      Bare minimum safety net."} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("output missing %q:\n%s", want, buf.String())
		}
	}
	if !u.confirm("next") {
		t.Error("bytes after enter were swallowed instead of left for confirm")
	}
	if !items[0].Checked {
		t.Error("pick changed its input")
	}

	stdin = strings.NewReader("\x1b")
	if _, ok := u.pick("t", "h", items); ok {
		t.Error("esc did not cancel")
	}
	stdin = strings.NewReader("q")
	if _, ok := u.pick("t", "h", items); ok {
		t.Error("q did not cancel")
	}
	stdin = strings.NewReader("")
	if _, ok := u.pick("t", "h", items); ok {
		t.Error("EOF did not cancel")
	}
}

func TestWrap(t *testing.T) {
	got := wrap("aa bb cc dd", 5)
	if !reflect.DeepEqual(got, []string{"aa bb", "cc dd"}) {
		t.Errorf("wrap: %q", got)
	}
	if got := wrap("", 5); got != nil {
		t.Errorf("wrap empty: %q", got)
	}
}
