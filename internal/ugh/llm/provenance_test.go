package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSourceDefaultsFromRoleAndExplicitTagWins(t *testing.T) {
	if got := (Message{Role: RoleTool}).Source(); got != SourceToolOutput {
		t.Errorf("tool = %q", got)
	}
	if got := (Message{Role: RoleUser}).Source(); got != SourceUser {
		t.Errorf("user = %q", got)
	}
	if got := (Message{Role: RoleAssistant}).Source(); got != "" {
		t.Errorf("assistant = %q", got)
	}
	m := Message{Role: RoleUser}.WithSource(SourceHarness)
	if m.Source() != SourceHarness || m.Untrusted() {
		t.Errorf("tagged = %+v", m)
	}
	if !(Message{Role: RoleTool}).Untrusted() {
		t.Error("tool output must be untrusted")
	}
}

func TestWithSourceDoesNotAliasMeta(t *testing.T) {
	a := Message{Role: RoleTool, Meta: map[string]string{"k": "v"}}
	b := a.WithSource(SourceUser)
	if _, ok := a.Meta[MetaSource]; ok || b.Meta["k"] != "v" {
		t.Errorf("a=%v b=%v", a.Meta, b.Meta)
	}
}

func TestMetaRoundTripsThroughJSON(t *testing.T) {
	raw, err := json.Marshal(Message{Role: RoleTool, Text: "x"}.WithSource(SourceToolOutput))
	if err != nil {
		t.Fatal(err)
	}
	var back Message
	if err := json.Unmarshal(raw, &back); err != nil || back.Meta[MetaSource] != SourceToolOutput {
		t.Fatalf("back = %+v err=%v", back, err)
	}
}

func TestCachePartsNeverCarryMeta(t *testing.T) {
	m := Message{Role: RoleTool, Text: "out", ToolCallID: "1"}.WithSource(SourceToolOutput)
	raw, _ := json.Marshal(WithTextContentParts(m, true, "5m"))
	if strings.Contains(string(raw), "tool_output") {
		t.Errorf("wire form leaked metadata: %s", raw)
	}
}
