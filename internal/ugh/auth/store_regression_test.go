package auth

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestResolveWhileProvidersRegisterDoesNotRace(t *testing.T) {
	s := NewStore(&fakeBackend{name: "fake"})
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			RegisterProvider(fmt.Sprintf("race-test-%d", i), "")
		}()
		go func() {
			defer wg.Done()
			Resolve(fmt.Sprintf("race-test-%d", i), nil, s)
		}()
	}
	wg.Wait()
}

func TestMaskKeyNeverShowsAShortKey(t *testing.T) {
	for n := 0; n <= 13; n++ {
		for _, unit := range []string{"a", "é"} {
			key := strings.Repeat(unit, n)
			got := MaskKey(key)
			if n > 0 && n < 12 && got != "…" || n >= 12 && strings.Contains(got, key) {
				t.Errorf("MaskKey(%q) = %q", key, got)
			}
			if n == 0 && got != "" {
				t.Errorf("MaskKey(\"\") = %q", got)
			}
		}
	}
}
