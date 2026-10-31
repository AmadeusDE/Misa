package session

import "testing"

func TestCommandSubstitutionExpansion(t *testing.T) {
	runExpansionCases(t, []expansionCase{
		{name: "failed assignment status", src: `x=$(false); printf '%s:<%s>' "$?" "$x"`, out: "1:<>"},
		{name: "last substitution status", src: `x=$(false) y=$(true); printf '%s:' "$?"; x=$(true) y=$(false); printf '%s' "$?"`, out: "0:1"},
		{name: "nested quotes", src: `x=$(printf '%s' "$(printf '%s' 'a b')"); printf '<%s>' "$x"`, out: "<a b>"},
		{name: "trim trailing newlines", src: `x=$(printf 'a\n\nb\n\n'); printf '<%s>' "$x"`, out: "<a\n\nb>"},
	})
}
