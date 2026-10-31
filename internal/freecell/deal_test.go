package freecell

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func TestV1DealVectors(t *testing.T) {
	// SHA-256 of JSON columns, including every physical ID/rank/suit in deal order.
	for _, tc := range []struct {
		seed uint64
		want string
	}{
		{0, "ab86548d65212eab53e60b906a067d6ee7d8cfc4349d72354e870b1db7a8b87a"},
		{42, "0514b57ed7d902f6ab54a549fb10f38187736098737489184e6e082adeb476dd"},
		{^uint64(0), "f989ee5f784302ca621d907a2b4af2ac8fe14d05492b2ba1022db4dd09ec9d55"},
	} {
		s := NewState(tc.seed)
		data, err := json.Marshal(s.Columns)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != tc.want {
			t.Fatalf("v1 seed %d: %s", tc.seed, got)
		}
	}
}
