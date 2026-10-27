package builtins

import (
	"testing"
)

func TestParseLongPrefixMatch(t *testing.T) {
	spec, err := ParseSpec("v(verbose)n(number)b(number-nonblank)s(squeeze-blank)e(show-ends)T(show-tabs)A(show-all)t(show-nonprinting)")
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantKey string
		wantErr bool
	}{
		{"exact match", []string{"--verbose"}, "v", false},
		{"prefix unique", []string{"--verb"}, "v", false},
		{"prefix show-non", []string{"--show-non"}, "t", false},
		{"prefix show-ta", []string{"--show-ta"}, "T", false},
		{"prefix show-e", []string{"--show-e"}, "e", false},
		{"prefix squeeze", []string{"--squeeze"}, "s", false},
		{"prefix show-a", []string{"--show-a"}, "A", false},
		{"prefix number-non", []string{"--number-non"}, "b", false},
		{"ambiguous prefix show-", []string{"--show-"}, "", true},
		{"unknown prefix", []string{"--zzz"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := spec.ParseArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantKey != "" && !r.Bool(tt.wantKey) {
				t.Errorf("expected key %q to be set", tt.wantKey)
			}
		})
	}
}

func TestParseLongPrefixWithValue(t *testing.T) {
	spec, err := ParseSpec("w(wrap):")
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	r, err := spec.ParseArgs([]string{"--wr", "76"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, ok := r.String("w")
	if !ok || v != "76" {
		t.Errorf("expected wrap=76, got %q ok=%v", v, ok)
	}

	r, err = spec.ParseArgs([]string{"--wr=42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, ok = r.String("w")
	if !ok || v != "42" {
		t.Errorf("expected wrap=42, got %q ok=%v", v, ok)
	}
}
