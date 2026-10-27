package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

func TestInitWritesExampleOnceAndKeepsEdits(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := writeExample(dir, &out); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "ughbot.toml")
	if _, err := config.Load(cfgPath); err != nil {
		t.Fatalf("written example does not load: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := writeExample(dir, &out); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(cfgPath); string(got) != "edited" {
		t.Fatalf("init overwrote the config: %q", got)
	}
	if !strings.Contains(out.String(), "kept") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestGroupCost(t *testing.T) {
	rows := []api.CostRow{{Bot: "a", Day: "d1", USD: 1}, {Bot: "a", Day: "d2", USD: 2}, {Bot: "b", Day: "d1", USD: 4}}
	byBot, _ := groupCost(rows, "bot")
	if len(byBot) != 2 || byBot[0].Bot != "a" || byBot[0].USD != 3 {
		t.Fatalf("by bot = %v", byBot)
	}
	byDay, _ := groupCost(rows, "day")
	if len(byDay) != 2 || byDay[0].Day != "d1" || byDay[0].USD != 5 {
		t.Fatalf("by day = %v", byDay)
	}
	if _, err := groupCost(rows, "week"); err == nil {
		t.Fatal("bad --by accepted")
	}
}
