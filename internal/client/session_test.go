package client

import "testing"

func TestBootSessionIDFromIdentityDeterministic(t *testing.T) {
	identity := "host-a:1708200000"
	first := bootSessionIDFromIdentity(identity)
	second := bootSessionIDFromIdentity(identity)
	if first != second {
		t.Fatalf("expected deterministic session id for same identity; got %q vs %q", first, second)
	}
}

func TestBootSessionIDFromIdentityChanges(t *testing.T) {
	a := bootSessionIDFromIdentity("host-a:1708200000")
	b := bootSessionIDFromIdentity("host-a:1708209999")
	if a == b {
		t.Fatalf("expected different session ids for different boot identities; both were %q", a)
	}
}
