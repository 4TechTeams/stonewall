package sandbox

import (
	"os"
	"sort"
	"strings"
)

// readlink is replaced in tests to simulate merged-usr hosts.
var readlink = os.Readlink

// systemDirs are exposed read-only. On merged-usr hosts some are symlinks and are recreated as such.
var systemDirs = []string{"/usr", "/etc", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/opt"}

// BwrapArgs renders the plan as bubblewrap arguments. Later mounts override earlier ones.
func BwrapArgs(p *Plan) []string {
	a := []string{"--unshare-pid", "--die-with-parent", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp"}
	for _, d := range systemDirs {
		if target, err := readlink(d); err == nil {
			a = append(a, "--symlink", target, d)
		} else {
			a = append(a, "--ro-bind-try", d, d)
		}
	}
	for _, d := range []string{"/run/systemd/resolve", "/run/resolvconf", "/run/NetworkManager"} { // resolv.conf symlink targets on common distros
		a = append(a, "--ro-bind-try", d, d)
	}
	a = append(a, "--tmpfs", p.Home)
	a = append(a, "--bind", p.Project, p.Project)
	for _, e := range p.ExposeWrite {
		a = append(a, "--bind", e, e)
	}
	for _, e := range p.ExposeRead {
		a = append(a, "--ro-bind", e, e)
	}
	for _, r := range p.Readonly {
		a = append(a, "--ro-bind", r, r)
	}
	// ReadonlyFiles needs nothing here: outside the project it is either never mounted or already
	// read-only, because only the system directories above are bound in.
	for _, h := range p.HiddenDirs {
		a = append(a, "--tmpfs", h)
	}
	for _, h := range p.HiddenFiles {
		a = append(a, "--ro-bind", "/dev/null", h)
	}
	for _, path := range sortedValues(p.Bins) {
		if !underSystemDir(path) { // e.g. binaries under $HOME, otherwise hidden by the tmpfs
			a = append(a, "--ro-bind", path, path)
		}
	}
	a = append(a, "--ro-bind", p.BinDir, p.BinDir)
	a = append(a, "--setenv", "PATH", p.BinDir, "--chdir", p.Cwd, "--")
	return append(a, p.Argv...)
}

func underSystemDir(path string) bool {
	for _, d := range systemDirs {
		if strings.HasPrefix(path, d+"/") {
			return true
		}
	}
	return false
}

// sortedValues returns the distinct values of m in sorted order, for deterministic output.
func sortedValues(m map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range m {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
