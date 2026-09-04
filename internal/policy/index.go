package policy

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// IndexURL lists the official policies. A variable so tests point it at a test server.
var IndexURL = "https://stonewall.sh/policies/_index.yml"

// IndexEntry is one official policy as the index describes it.
type IndexEntry struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
}

// Index fetches IndexURL under the rules of a policy download and returns its entries in order. Entries
// without a url are dropped: they cannot be included.
func (l Loader) Index() ([]IndexEntry, error) {
	u, err := remoteURL(IndexURL)
	if err != nil {
		return nil, err
	}
	body, err := l.fetch(u)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	var idx struct {
		Policies []IndexEntry `yaml:"policies"`
	}
	if err := yaml.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("%s: %w", u, err)
	}
	var out []IndexEntry
	for _, e := range idx.Policies {
		if e.URL != "" {
			out = append(out, e)
		}
	}
	return out, nil
}
