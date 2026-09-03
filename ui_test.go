package main

import (
	"bytes"
	"errors"
	"os"
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
	u.block("stonewall will create .stonewall.yml", "include:\n  - https://stonewall.sh/policy/base.yml\n")
	want := "stonewall will create .stonewall.yml\n" +
		"  include:\n" +
		"    - https://stonewall.sh/policy/base.yml\n"
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
