package chess

import (
	"reflect"
	"strings"
	"testing"
)

func TestAtomicVariantSchemaBuiltin(t *testing.T) {
	vs, errs := Builtins()
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	v, err := Find(vs, "atomic")
	if err != nil {
		t.Fatal(err)
	}
	base, err := Find(vs, "chess")
	if err != nil {
		t.Fatal(err)
	}
	if v.Files != 8 || v.Ranks != 8 || v.Setup != base.Setup || !reflect.DeepEqual(v.Castling, base.Castling) || len(v.Pieces) != len(base.Pieces) {
		t.Fatal("atomic did not inherit chess board and pieces")
	}
	k := v.Pieces[v.TypeOf('K')-1]
	if k.Name != "King" || k.Betza != "mK" || !k.Royal || k.Glyph != "♚" {
		t.Fatalf("king: %+v", k)
	}
	if v.Explosion == nil || v.Explosion.Radius != 1 || !reflect.DeepEqual(v.Explosion.Immune, []uint8{v.TypeOf('P')}) || !reflect.DeepEqual(v.Extinction, []uint8{v.TypeOf('K')}) {
		t.Fatalf("rules: %+v %v", v.Explosion, v.Extinction)
	}
}

func TestAtomicVariantSchemaInheritance(t *testing.T) {
	vs, err := parseVariants(map[string][]byte{
		"1.yaml": []byte("id: inherited\nextends: atomic\n"),
		"2.yaml": []byte("id: cleared\nextends: inherited\nexplosion: {radius: 0}\nextinction: []\n"),
		"3.yaml": []byte("id: alltypes\nextends: cleared\nextinction: all\n"),
		"4.yaml": []byte("id: star\nextends: cleared\nextinction: '*'\n"),
		"5.yaml": []byte("id: replaced\nextends: atomic\nexplosion: {radius: 2}\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	get := func(name string) *Variant {
		v, err := Find(vs, name)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	inherited := get("inherited")
	if inherited.Explosion == nil || inherited.Explosion.Radius != 1 || len(inherited.Extinction) != 1 {
		t.Fatal("rules not inherited")
	}
	cleared := get("cleared")
	if cleared.Explosion != nil || len(cleared.Extinction) != 0 {
		t.Fatal("rules not cleared")
	}
	for _, name := range []string{"alltypes", "star"} {
		v := get(name)
		if len(v.Extinction) != len(v.Pieces) {
			t.Fatalf("%s: %v", name, v.Extinction)
		}
		for i, typ := range v.Extinction {
			if int(typ) != i+1 {
				t.Fatalf("%s: %v", name, v.Extinction)
			}
		}
	}
	replaced := get("replaced")
	if replaced.Explosion == nil || replaced.Explosion.Radius != 2 || len(replaced.Explosion.Immune) != 0 {
		t.Fatal("explosion should replace entire inherited field")
	}
}

func TestAtomicVariantSchemaErrors(t *testing.T) {
	for _, field := range []string{
		"explosion: {radius: -1}", "explosion: {radius: 1, immune: [Z]}",
		"explosion: {radius: 1, immune: [P, p]}", "extinction: [Z]", "extinction: [K, k]", "extinction: K", "extinction: [all]", "explosion: {radius: nope}",
	} {
		t.Run(field, func(t *testing.T) {
			_, err := parseVariants(map[string][]byte{"bad.yaml": []byte("id: bad\nextends: chess\n" + field + "\n")})
			if err == nil || !strings.Contains(err.Error(), "bad.yaml") {
				t.Fatalf("expected source error, got %v", err)
			}
		})
	}
}
