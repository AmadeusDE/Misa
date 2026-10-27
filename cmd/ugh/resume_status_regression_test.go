package main

import (
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/session"
)

func TestResumedAgentAcceptsSendAndWatchDropsOldResult(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s, err := session.Create("/w", session.Record{Run: "resumed-run", Agent: "root/worker-1", Parent: "root"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	appendRecord := func(r session.Record) {
		t.Helper()
		if err := s.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	appendRecord(session.Record{Type: "result", Status: "ok", Turns: 61})
	tl := &tail{path: s.Path()}
	if err := tl.poll(nil); err != nil {
		t.Fatal(err)
	}
	if tl.status() != "ok" {
		t.Fatalf("finished status=%s", tl.status())
	}
	if err := sendCmd([]string{"--session", s.Path(), "root/worker-1", "check output"}); err == nil {
		t.Fatal("finished agent accepted send")
	}
	for _, status := range []string{"queued", "running"} {
		appendRecord(session.Record{Type: "status", Status: status})
		if err := tl.poll(nil); err != nil {
			t.Fatal(err)
		}
		if tl.result != nil || tl.status() == "ok" || tl.status() == "waiting" {
			t.Fatalf("resumed %s status=%s result=%v", status, tl.status(), tl.result)
		}
		if _, done := lastResult(s.Path()); done {
			t.Fatal("lastResult retained an earlier execution")
		}
		if err := sendCmd([]string{"--session", s.Path(), "root/worker-1", "check output"}); err != nil {
			t.Fatalf("active continuation rejected send: %v", err)
		}
	}
	inbox, _ := session.InboxPath("resumed-run", "root/worker-1")
	messages, _ := session.ReadInbox(inbox, 0)
	if len(messages) != 2 {
		t.Fatalf("inbox=%v", messages)
	}
	appendRecord(session.Record{Type: "message", Message: &llm.Message{Role: llm.RoleUser, Text: "continue"}})
	appendRecord(session.Record{Type: "result", Status: "error", Turns: 63})
	if err := tl.poll(nil); err != nil {
		t.Fatal(err)
	}
	if tl.status() != "error" {
		t.Fatalf("new completion status=%s", tl.status())
	}
	if err := sendCmd([]string{"--session", s.Path(), "root/worker-1", "check output"}); err == nil || !strings.Contains(err.Error(), "already finished") {
		t.Fatalf("completed continuation send: %v", err)
	}
}
