package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
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

// pickItem is one row of pick: a label, a dimmed detail paragraph under it, and whether it starts ticked.
type pickItem struct {
	Label, Detail string
	Checked       bool
}

// interactive reports whether stdin can drive the picker: a terminal, or a test's reader.
func interactive() bool {
	f, ok := stdin.(*os.File)
	return !ok || term.IsTerminal(int(f.Fd()))
}

// rawMode puts a terminal stdin into raw mode and returns the restore function; a test's reader gets a no-op.
func rawMode() (func(), error) {
	f, ok := stdin.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return func() {}, nil
	}
	state, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		return nil, err
	}
	return func() { term.Restore(int(f.Fd()), state) }, nil
}

// width is the terminal width of u.w, or 80 when it is not a terminal.
func (u ui) width() int {
	if f, ok := u.w.(*os.File); ok {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 20 {
			return w
		}
	}
	return 80
}

// keyReader hands out one key per call: a byte, or an arrow escape sequence whole. It reads whatever a
// terminal delivers per key press; what a test feeds beyond one key is kept for the next call, and the
// picker gives the rest back to stdin when it returns, so a prompt after it reads the right bytes.
type keyReader struct {
	r   io.Reader
	buf []byte
}

func (k *keyReader) next() (string, error) {
	if len(k.buf) == 0 {
		var b [32]byte
		n, err := k.r.Read(b[:])
		if n == 0 {
			if err == nil {
				err = io.EOF
			}
			return "", err
		}
		k.buf = append(k.buf, b[:n]...)
	}
	n := 1
	if k.buf[0] == 0x1b && len(k.buf) >= 3 && k.buf[1] == '[' && (k.buf[2] == 'A' || k.buf[2] == 'B') {
		n = 3
	}
	key := string(k.buf[:n])
	k.buf = k.buf[n:]
	return key, nil
}

// pick shows items as a checkbox list under title and hint and returns which are ticked when Enter is
// pressed. ok is false when the user cancels with Esc, q, Ctrl-C or end of input. Up/k and Down/j move,
// Space toggles. The block is redrawn in place on every key.
func (u ui) pick(title, hint string, items []pickItem) (checked []bool, ok bool) {
	restore, err := rawMode()
	if err != nil {
		return nil, false
	}
	defer restore()
	keys := &keyReader{r: stdin}
	defer func() {
		if len(keys.buf) > 0 {
			stdin = io.MultiReader(bytes.NewReader(keys.buf), stdin)
		}
	}()

	checked = make([]bool, len(items))
	for i, it := range items {
		checked[i] = it.Checked
	}
	cur, lines, width := 0, 0, u.width()
	render := func() {
		var b strings.Builder
		if lines > 0 {
			fmt.Fprintf(&b, "\x1b[%dA\x1b[J", lines) // back to the top of the block and clear it
		}
		lines = 0
		put := func(s string) { b.WriteString(s + "\r\n"); lines++ } // raw mode does not translate \n
		put(u.paint("1;38;5;202", "stonewall") + "  " + title)
		put(strings.Repeat(" ", len("stonewall")) + "  " + u.paint("2", hint))
		put("")
		for i, it := range items {
			mark, arrow := "[ ]", "  "
			if checked[i] {
				mark = u.paint("1;38;5;202", "[x]")
			}
			if i == cur {
				arrow = u.paint("1;38;5;202", ">") + " "
			}
			put(arrow + mark + " " + u.paint("1", it.Label))
			for _, l := range wrap(it.Detail, width-6) {
				put("      " + u.paint("2", l))
			}
		}
		fmt.Fprint(u.w, b.String())
	}
	for {
		render()
		key, err := keys.next()
		if err != nil {
			return nil, false
		}
		switch key {
		case "\x1b[A", "k":
			if cur > 0 {
				cur--
			}
		case "\x1b[B", "j":
			if cur < len(items)-1 {
				cur++
			}
		case " ":
			checked[cur] = !checked[cur]
		case "\r", "\n":
			return checked, true
		case "\x1b", "q", "\x03":
			return nil, false
		}
	}
}

// wrap breaks s at spaces into lines of at most width characters; a longer single word stays whole.
func wrap(s string, width int) []string {
	var out []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len(line)+1+len(w) > width {
			out = append(out, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}
