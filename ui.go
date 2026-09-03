package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/4TechTeams/stonewall/v2/internal/policy"
)

// stdin is where confirm reads the answer. Tests swap it.
var stdin io.Reader = os.Stdin

// ui renders stonewall's own messages. In plain mode: no colour, one "stonewall: key value" line per row.
type ui struct {
	w     io.Writer
	plain bool
	color bool
}

// newUI picks plain mode when asked or when w is not a terminal; colour additionally needs NO_COLOR unset.
func newUI(w io.Writer, plain bool) ui {
	if f, ok := w.(*os.File); ok && !plain {
		fi, err := f.Stat()
		plain = err != nil || fi.Mode()&os.ModeCharDevice == 0
	}
	return ui{w: w, plain: plain, color: !plain && os.Getenv("NO_COLOR") == ""}
}

func (u ui) paint(code, s string) string {
	if !u.color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// rows prints key/value rows: aligned under one stonewall header, or one plain line each.
func (u ui) rows(kv ...[2]string) {
	for i, r := range kv {
		if u.plain {
			fmt.Fprintf(u.w, "stonewall: %s %s\n", r[0], r[1])
			continue
		}
		head := strings.Repeat(" ", len("stonewall"))
		if i == 0 {
			head = u.paint("1;38;5;202", "stonewall")
		}
		fmt.Fprintf(u.w, "%s  %s  %s\n", head, u.paint("2", fmt.Sprintf("%-8s", r[0])), r[1])
	}
}

// warn prints a launch-time advisory: a continuation row aligned under the status block.
func (u ui) warn(msg string) {
	if u.plain {
		fmt.Fprintln(u.w, "stonewall: warning", msg)
		return
	}
	head := strings.Repeat(" ", len("stonewall"))
	fmt.Fprintf(u.w, "%s  %s  %s\n", head, u.paint("1;33", fmt.Sprintf("%-8s", "warning")), msg)
}

// block prints a title and the text it introduces, indented, for the user to read before answering.
func (u ui) block(title, body string) {
	fmt.Fprintln(u.w, u.paint("1", title))
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		fmt.Fprintln(u.w, "  "+u.paint("2", line))
	}
}

// confirm asks question and reads one line from stdin. Only y or yes is a yes; EOF is a no.
func (u ui) confirm(question string) bool {
	fmt.Fprint(u.w, u.paint("1", question)+" [y/N] ")
	answer := strings.ToLower(strings.TrimSpace(readLine(stdin)))
	return answer == "y" || answer == "yes"
}

// readLine reads up to the next newline one byte at a time, so nothing is buffered past the answer.
func readLine(r io.Reader) string {
	var line []byte
	var c [1]byte
	for {
		n, err := r.Read(c[:])
		if n > 0 {
			if c[0] == '\n' {
				break
			}
			line = append(line, c[0])
		}
		if err != nil {
			break
		}
	}
	return string(line)
}

func (u ui) error(err error) {
	if u.plain {
		fmt.Fprintln(u.w, "stonewall: error", err)
		return
	}
	fmt.Fprintf(u.w, "%s  %s  %s\n", u.paint("1;38;5;202", "stonewall"), u.paint("1;31", "error   "), err)
}

// help renders the styled help page: shown for `stonewall --help` and when run without arguments.
func (u ui) help() string {
	orange := func(s string) string { return u.paint("1;38;5;202", s) }
	dim := func(s string) string { return u.paint("2", s) }
	bold := func(s string) string { return u.paint("1", s) }

	var b strings.Builder

	fmt.Fprintf(&b, "%s %s\n\n", orange("stonewall"), dim(version+"  kernel-enforced sandbox for AI coding agents"))

	fmt.Fprintf(&b, "%s\n", orange("USAGE"))
	b.WriteString("  stonewall [options] <agent> [agent args...]\n\n")
	b.WriteString("  Launches <agent> in the current project inside a sandbox. What the agent can see, change and run is\n")
	b.WriteString("  exactly what .stonewall.yml and its includes allow; your home directory is hidden unless exposed.\n")
	b.WriteString("  Same directory, same tools. Options go before the agent; everything after it is passed through.\n\n")

	fmt.Fprintf(&b, "%s\n", orange("EXAMPLES"))
	example := func(cmd, desc string) {
		fmt.Fprintf(&b, "  %s%s%s\n", bold(cmd), strings.Repeat(" ", 34-len(cmd)), desc)
	}
	example("stonewall claude", "Claude Code in this project")
	example("stonewall claude --resume", "agent arguments pass through untouched")
	example("stonewall -n claude", "print the bwrap / sandbox-exec command, launch nothing")
	example("stonewall -p ci.yml codex", "use another policy file")
	example("stonewall sh -c 'ls ~'", "look around from the inside")
	b.WriteString("\n")

	fmt.Fprintf(&b, "%s\n", orange("OPTIONS"))
	option := func(short, long, desc string) {
		spelling := short + long
		fmt.Fprintf(&b, "  %s%s%s\n", bold(spelling), strings.Repeat(" ", 20-len(spelling)), desc)
	}
	option("-p, ", "--policy FILE", "policy file (default: <project root>/"+policy.FileName+")")
	option("-n, ", "--dry-run", "print the sandbox command and exit")
	option("    ", "--plain", "no colour or formatting in stonewall's own output")
	option("-v, ", "--version", "print the version")
	option("-h, ", "--help", "show this help")
	b.WriteString("\n")

	fmt.Fprintf(&b, "%s\n", orange("COMMANDS"))
	command := func(name, desc string) {
		fmt.Fprintf(&b, "  %s%s%s\n", bold(name), strings.Repeat(" ", 20-len(name)), desc)
	}
	command("policy update", "refresh cached remote policies: fetch every https include, show a diff for")
	fmt.Fprintln(&b, strings.Repeat(" ", 22)+"changed ones and ask before replacing them")
	b.WriteString("\n")

	fmt.Fprintf(&b, "  %s\n", dim("https://github.com/4TechTeams/stonewall"))

	return b.String()
}
