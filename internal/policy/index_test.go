package policy

import (
	"reflect"
	"strings"
	"testing"
)

func TestIndex(t *testing.T) {
	srv := newPolicyServer(t, "policies:\n  - name: Base\n    url: https://stonewall.sh/policies/base.yml\n    description: Bare minimum.\n  - name: Local\n    description: no url, not includable\n")
	defer func(u string) { IndexURL = u }(IndexURL)
	IndexURL = srv.URL + "/_index.yml"
	l := Loader{Client: srv.Client()}

	got, err := l.Index()
	if err != nil {
		t.Fatal(err)
	}
	want := []IndexEntry{{Name: "Base", URL: "https://stonewall.sh/policies/base.yml", Description: "Bare minimum."}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("index: got %+v want %+v", got, want)
	}

	srv.body = "policies: [oops"
	if _, err := l.Index(); err == nil {
		t.Fatal("garbage accepted")
	}
	srv.body = strings.Repeat("x", maxInclude+1)
	if _, err := l.Index(); err == nil {
		t.Fatal("oversized index accepted")
	}
	IndexURL = "http://stonewall.sh/policies/_index.yml"
	if _, err := l.Index(); err == nil {
		t.Fatal("plain http accepted")
	}
}
