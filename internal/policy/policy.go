// Package policy loads and merges Stonewall sandbox policies.
package policy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the repo-local policy file, found at the project root.
const FileName = ".stonewall.yml"

// Policy is a parsed .stonewall.yml. All fields are optional.
type Policy struct {
	Include []string `yaml:"include,omitempty"` // policies applied before this file, in order; local paths or https:// URLs
	Bin     Bin      `yaml:"bin"`
	Project Project  `yaml:"project"`
	Expose  Expose   `yaml:"expose"`
}

// Bin controls what is on PATH inside the sandbox.
type Bin struct {
	Allowed []string `yaml:"allowed"`          // program names on PATH
	Denied  []string `yaml:"denied,omitempty"` // program names removed from an earlier source's allowed
}

// Project holds restrictions on paths relative to the project root.
type Project struct {
	Hidden   []string `yaml:"hidden"`             // content unreadable
	Readonly []string `yaml:"readonly"`           // read but not change
	Writable []string `yaml:"writable,omitempty"` // paths released from an earlier source's hidden or readonly
}

// Expose grants access to host paths outside the project (~/ or absolute).
type Expose struct {
	Read  []string `yaml:"read"`
	Write []string `yaml:"write"`          // read-write
	None  []string `yaml:"none,omitempty"` // paths whose exposure by an earlier source is withdrawn
}

// FindRoot walks up from dir and returns the nearest directory holding FileName or .git, else dir itself.
func FindRoot(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if exists(filepath.Join(d, FileName)) || exists(filepath.Join(d, ".git")) {
			return d
		}
		if filepath.Dir(d) == d {
			return dir
		}
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Load reads and parses a policy file. It does not resolve includes; see Loader.
func Load(path string) (Policy, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, err
	}
	return Parse(b)
}

// Parse decodes YAML. Unknown keys are an error; an empty document is an empty policy.
// Every expose entry must start with ~/ or /.
func Parse(b []byte) (Policy, error) {
	var p Policy
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return Policy{}, err
	}
	for _, list := range [][]string{p.Expose.Read, p.Expose.Write, p.Expose.None} {
		for _, e := range list {
			if !strings.HasPrefix(e, "~/") && !strings.HasPrefix(e, "/") {
				return Policy{}, fmt.Errorf("expose: %q must start with ~/ or /", e)
			}
		}
	}
	return p, nil
}

// Scaffold returns the policy proposed for a project whose first launch runs agent.
func Scaffold(agent string) string {
	inc := "  - https://stonewall.sh/policy/base.yml\n"
	if strings.HasPrefix(filepath.Base(agent), "claude") {
		inc += "  - https://stonewall.sh/policy/claude.yml\n"
	}
	return `# Stonewall sandbox policy. Nothing outside this file and its includes is ever allowed.
# Includes are applied in order; this file is applied last and wins.
include:
` + inc + `# project:               # paths relative to this file
#   readonly:
#     - .git
#   hidden:              # content unreadable inside the sandbox
#     - .env
#   writable:            # undo an included hidden/readonly rule
#     - docs
# bin:
#   allowed:             # programs on PATH. Interpreters (bash, python) weaken this.
#     - make
#   denied:              # remove a program an include allowed
#     - bash
# expose:                # host paths outside the project, ~/ or absolute. $HOME is hidden otherwise.
#   read:
#     - ~/.npm
#   write:               # read-write
#     - ~/.cache
#   none:                # remove an exposure an include granted
#     - ~/.npm
`
}

// WriteScaffold creates path holding content. It fails if path exists.
func WriteScaffold(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
