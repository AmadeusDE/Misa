package chess

import (
	"encoding/json"
	"os"
	"testing"
)

type difficultyPosition struct {
	ID, Split string
	Moves     []string
}

func difficultyCorpus(t *testing.T) []difficultyPosition {
	t.Helper()
	f, err := os.Open("testdata/difficulty/positions.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var positions []difficultyPosition
	dec := json.NewDecoder(f)
	for dec.More() {
		var p difficultyPosition
		if err := dec.Decode(&p); err != nil {
			t.Fatal(err)
		}
		positions = append(positions, p)
	}
	return positions
}

func corpusMatch(t *testing.T, v *Variant, sample difficultyPosition) *Match {
	t.Helper()
	m := NewMatch(v)
	for _, text := range sample.Moves {
		found := false
		for _, mv := range m.Legal() {
			uci := v.SquareName(mv.From) + v.SquareName(mv.To)
			if mv.Promote != 0 {
				uci += string(v.Pieces[mv.Promote-1].Letter + 'a' - 'A')
			}
			if uci == text {
				m.Play(mv)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s: illegal corpus move %s", sample.ID, text)
		}
	}
	return m
}

func TestNormalHeldOutBaseline(t *testing.T) {
	data, err := os.Open("testdata/difficulty/baseline.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Close()
	baseline := map[string]Result{}
	dec := json.NewDecoder(data)
	for dec.More() {
		var row struct {
			ID     string
			Result Result
		}
		if err := dec.Decode(&row); err != nil {
			t.Fatal(err)
		}
		baseline[row.ID] = row.Result
	}
	v := builtin(t, "chess")
	positions := difficultyCorpus(t)
	if len(positions) < 100 || len(baseline) != len(positions) {
		t.Fatal("incomplete baseline corpus")
	}
	for _, sample := range positions {
		if sample.Split != "held-out" {
			continue
		}
		m := corpusMatch(t, v, sample)
		got := NewEngine(v).ThinkLevel(m.Position(), m.Hashes(), Normal, Limits{Depth: 4}, 0)
		if want, ok := baseline[sample.ID]; !ok || got != want {
			t.Errorf("%s: %+v, parent %+v", sample.ID, got, want)
		}
	}
}
