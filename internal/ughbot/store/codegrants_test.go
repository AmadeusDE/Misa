package store

import (
	"errors"
	"testing"
)

func TestCodeGrants(t *testing.T) {
	s, _ := open(t)
	for _, g := range []CodeGrant{{Bot: "a", Dir: "/r/one", Approval: 3}, {Bot: "a", Dir: "/r/two", Source: "https://x/two.git"},
		{Bot: "b", Dir: "/r/one"}, {Bot: "a", Dir: "/r/one", Approval: 9}} {
		if err := s.AddCodeGrant(g); err != nil {
			t.Fatal(err)
		}
	}
	a, err := s.CodeGrants("a")
	if err != nil || len(a) != 2 || a[0].Approval != 3 || a[1].Source != "https://x/two.git" {
		t.Fatalf("a = %+v, %v", a, err)
	}
	if all, _ := s.CodeGrants(""); len(all) != 3 {
		t.Fatalf("all = %+v", all)
	}
	if err := s.DeleteCodeGrant("a", "/r/one"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCodeGrant("a", "/r/one"); !errors.Is(err, ErrNoGrant) {
		t.Fatalf("second delete: %v", err)
	}
	if err := s.DeleteCodeGrants("a"); err != nil {
		t.Fatal(err)
	}
	if all, _ := s.CodeGrants(""); len(all) != 1 || all[0].Bot != "b" {
		t.Fatalf("after deleting a: %+v", all)
	}
}
