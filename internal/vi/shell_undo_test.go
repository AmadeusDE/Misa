package vi

import "testing"

func TestShellFilterMetadataUndo(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"same"}
	e.file.newEmpty = false
	e.file.eol = true
	e.replaceFilter([]byte("same"), 0, 0)
	if got := e.fileContent(); got != "same" || !e.modified {
		t.Fatalf("after filter %q modified %v", got, e.modified)
	}
	e.undo()
	if got := e.fileContent(); got != "same\n" || e.modified {
		t.Fatalf("undo %q modified %v", got, e.modified)
	}
	e.redo()
	if got := e.fileContent(); got != "same" || !e.modified {
		t.Fatalf("redo %q modified %v", got, e.modified)
	}
}

func TestShellReadEmptyBuffer(t *testing.T) {
	e := New("", t.TempDir())
	e.insertRead([]byte("new\n"), 0, "fixture")
	if got := e.fileContent(); got != "new\n" {
		t.Fatalf("read: %q", got)
	}
	e.undo()
	if got := e.fileContent(); got != "" {
		t.Fatalf("undo: %q", got)
	}
}

func TestGlobalSubstitutionMayContainShellLikeText(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"a", "b"}
	e.executeCommand("g#a#s/a/ ! /")
	if e.lines[0] != " ! " || e.lines[1] != "b" {
		t.Fatalf("lines: %q, status: %s", e.lines, e.statusMsg)
	}
}

func TestGlobalShellDetection(t *testing.T) {
	for _, command := range []string{"!true", "r !true", "read\t!true", "w !true", "write!\t!true", "wq !true"} {
		t.Run(command, func(t *testing.T) {
			if !isCompoundShell(command) {
				t.Fatalf("shell not detected: %q", command)
			}
		})
	}
	for _, command := range []string{"s/a/ ! /", "w file ! name", "read file ! name"} {
		t.Run(command, func(t *testing.T) {
			if isCompoundShell(command) {
				t.Fatalf("false shell detection: %q", command)
			}
		})
	}
}
