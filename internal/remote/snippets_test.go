package remote

import (
	"testing"
)

func TestSnippetParametersAreQuoted(t *testing.T) {
	v, e := ExpandSnippet("grep -- {{pattern}} {{file}}", map[string]string{"pattern": "a'; touch /tmp/unwanted; #", "file": "a b"})
	if e != nil || v != "grep -- 'a'\"'\"'; touch /tmp/unwanted; #' 'a b'" {
		t.Fatal(v, e)
	}
	if _, e = ExpandSnippet("{{missing}}", nil); e == nil {
		t.Fatal("missing parameter accepted")
	}
}
