package main

import (
	"fmt"
	"io"
	"os"
	"strings"
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

// block prints a title and, when there is one, the text it introduces, indented, for the user to read
// before answering.
func (u ui) block(title, body string) {
	fmt.Fprintln(u.w, u.paint("1", title))
	if body == "" {
		return
	}
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
