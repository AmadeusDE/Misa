package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestBrowserOversizedImageDoesNotOpenEditor(t *testing.T) {
	m, _, cmd, _ := browserFixture(t)
	configDir := filepath.Join(t.TempDir(), "misa")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(configDir))
	if err := os.WriteFile(filepath.Join(configDir, "imageviewer.yaml"), []byte("max_file_mib: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.dir, "huge.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(2 << 20); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	m.reloadFileList()
	selectBrowserName(t, m, "huge.png")
	m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.screen != screenFileBrowser || *cmd != nil || !strings.Contains(m.status, "limit") {
		t.Fatalf("unsafe file activation: screen=%v command=%v status=%q", m.screen, *cmd, m.status)
	}
}

func TestBrowserLimitedImageLaunchNotice(t *testing.T) {
	m, _, cmd, cb := browserFixture(t)
	configDir := filepath.Join(t.TempDir(), "misa")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(configDir))
	if err := os.WriteFile(filepath.Join(configDir, "imageviewer.yaml"), []byte("max_images: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	selectBrowserName(t, m, "z ;$.png")
	m.updateFileBrowser(tea.KeyPressMsg{Code: tea.KeyEnter})
	if *cmd == nil || !containsImageViewerEnv((*cmd).Env, "MISA_IMGVIEW_LIMITED=1") {
		t.Fatal("limited collection notice not passed to standalone viewer")
	}
	selected := filepath.Join(m.dir, "z ;$.png")
	if len((*cmd).Args) != 6 || (*cmd).Args[3] != selected || (*cmd).Args[5] != selected {
		t.Fatalf("selected file beyond MaxImages was not retained: %q", (*cmd).Args)
	}
	m.updateImageViewerFinished((*cb)(nil).(imageViewerFinished))
	if m.screen != screenFileBrowser || !strings.Contains(m.status, "limit reached") {
		t.Fatalf("collection warning lost on return: screen=%v status=%q", m.screen, m.status)
	}
}
