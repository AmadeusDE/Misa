package klondike

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func TestV1DealVectors(t *testing.T) {
	// SHA-256 of JSON [columns, hidden counts, stock, draw], including every ID/rank/suit.
	for _, tc := range []struct {
		seed uint64
		draw int
		want string
	}{
		{0, 1, "edabca9a2acacd3b46d4f7142664a17587bc64a273d6411218c72aee4cadb4ef"},
		{0, 3, "911f9282c57d2d1f7b968a54161b7ed73565ea5a8c3a9b24e3af6dc3768343c6"},
		{42, 1, "a15fb2790eff77068fc686ff256fe1badcd293ac896c1a937d05a8f217a5b1b5"},
		{42, 3, "5a47cbff5bd10d43b3a918298d829e5ce23f73e114703f76f39f73f56922e748"},
		{^uint64(0), 1, "6376642f535fdb25da4bb6b8717a310a44e41d39591ddd7702854142bc03a797"},
		{^uint64(0), 3, "c217ef661d4ea5622546a2e258627a23e9ac45a0e6c3d36d7ce8c6bc83789b2a"},
	} {
		s := NewState(tc.seed, tc.draw)
		data, err := json.Marshal([]any{s.Columns, s.Hidden, s.Stock, s.Draw})
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != tc.want {
			t.Fatalf("v1 seed %d draw %d: %s", tc.seed, tc.draw, got)
		}
	}
}
