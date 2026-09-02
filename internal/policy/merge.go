package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Merge applies sources in order. A later source overrides an earlier one per program or path; the negative
// lists remove. The result holds only the positive lists, in first-seen order, and no Include.
func Merge(sources ...Policy) (Policy, error) {
	home, _ := os.UserHomeDir()
	bin := newSet(func(s string) string { return s })
	project := newSet(filepath.Clean)
	expose := newSet(func(s string) string {
		if strings.HasPrefix(s, "~/") {
			s = filepath.Join(home, s[2:])
		}
		return filepath.Clean(s) // so a trailing slash cannot evade a negation or the conflict check
	})
	for _, src := range sources {
		for _, g := range []struct {
			set   *set
			list  string
			items []string
		}{
			{bin, "bin.allowed", src.Bin.Allowed},
			{bin, "bin.denied", src.Bin.Denied},
			{project, "project.hidden", src.Project.Hidden},
			{project, "project.readonly", src.Project.Readonly},
			{project, "project.writable", src.Project.Writable},
			{expose, "expose.read", src.Expose.Read},
			{expose, "expose.write", src.Expose.Write},
			{expose, "expose.none", src.Expose.None},
		} {
			for _, item := range g.items {
				if err := g.set.add(g.list, item); err != nil {
					return Policy{}, err
				}
			}
		}
		bin.endSource()
		project.endSource()
		expose.endSource()
	}
	return Policy{
		Bin: Bin{Allowed: bin.items("bin.allowed")},
		Project: Project{
			Hidden:   project.items("project.hidden"),
			Readonly: project.items("project.readonly"),
		},
		Expose: Expose{
			Read:  expose.items("expose.read"),
			Write: expose.items("expose.write"),
		},
	}, nil
}

// set classifies items into at most one list each, keeping first-seen order. Items are compared by a
// normalised key but reported with the text first seen. Negative lists are ordinary lists that items
// never come back out of.
type set struct {
	key   func(string) string
	order []string          // keys, first-seen order
	text  map[string]string // key -> text as first written
	list  map[string]string // key -> list it currently belongs to
	src   map[string]string // key -> list that claimed it in the source being applied
}

func newSet(key func(string) string) *set {
	return &set{key: key, text: map[string]string{}, list: map[string]string{}, src: map[string]string{}}
}

func (s *set) add(list, item string) error {
	k := s.key(item)
	if prev, ok := s.src[k]; ok && prev != list {
		return fmt.Errorf("%q listed in both %s and %s", item, prev, list)
	}
	s.src[k] = list
	if _, ok := s.text[k]; !ok {
		s.order = append(s.order, k)
		s.text[k] = item
	}
	s.list[k] = list
	return nil
}

// endSource clears the per-source conflict record, so the next source may reclassify freely.
func (s *set) endSource() { s.src = map[string]string{} }

func (s *set) items(list string) []string {
	var out []string
	for _, k := range s.order {
		if s.list[k] == list {
			out = append(out, s.text[k])
		}
	}
	return out
}
