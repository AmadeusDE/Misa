package spider

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func TestV1DealVectors(t *testing.T) {
	// SHA-256 of JSON [columns, hidden counts, stock], including every ID/rank/suit.
	for _, tc := range []struct {
		seed  uint64
		suits int
		want  string
	}{
		{0, 1, "f5966221c09f8afffbdd02c8090500379f19df33c1d8a3e8b5faf9eb3396cf9b"},
		{0, 2, "77527d7e31fe18b5f8fce472e094be4290476689b8563b52808d384354872b1e"},
		{0, 4, "f512c4da71afaa909610d7b4838ad62a9ce34add086b61d67ae4540a7ade9f88"},
		{42, 1, "03b616dcd5ac123a8d880e9131ef213f2f31fe36dea853c9300be31d8e5c3ef8"},
		{42, 2, "87a78e19688cb0005e5f0ccdf9d2a01a2fe776cabf40f631ee622aeeaced8704"},
		{42, 4, "6b9a654f322ae50279bc415dd406d1102d3509f82595eb973684de38582a6a84"},
		{^uint64(0), 1, "f3f8b821c12330c96ce50590359f452e608417d26de3aceabdf1420abc80022a"},
		{^uint64(0), 2, "e6b6516c430569f44ea1fdef82fb2d8060f76f8f3e93bb1dc36d2d7839da20d8"},
		{^uint64(0), 4, "dc7f2f6a414c065bb717f70ba287e8592d26f5809f430a2f96c0e0217d47edd4"},
	} {
		s := NewState(tc.seed, tc.suits)
		data, err := json.Marshal([]any{s.Columns, s.Hidden, s.Stock})
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != tc.want {
			t.Fatalf("v1 seed %d suits %d: %s", tc.seed, tc.suits, got)
		}
	}
}
