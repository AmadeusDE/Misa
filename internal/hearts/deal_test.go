package hearts

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func TestV1DealVectors(t *testing.T) {
	// SHA-256 of JSON [hands, opening seat, passing], including every ID/rank/suit.
	for _, tc := range []struct {
		seed uint64
		want string
	}{
		{0, "f48af719ad6f1df7323e9daca9e52db03dd703e133e6e57dcb0de2b6a19d0499"},
		{42, "cd33a34b1699c3d9e8fdf461b4a687667ff718e2f4b3d2b6565d5e1033a2bab5"},
		{^uint64(0), "5d93a9aefec66d4a138fce38b368e1bd4bde15be11aa46b3db1a2f2400e65c53"},
	} {
		s := NewState(tc.seed, 100)
		data, err := json.Marshal([]any{s.Hands, s.Turn, s.Passing})
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != tc.want {
			t.Fatalf("v1 seed %d: %s", tc.seed, got)
		}
	}
}
