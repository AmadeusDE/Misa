package help

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AmadeusDE/Misa/internal/hostpath"
)

func tldrDir() (string, error) {
	dir := hostpath.TldrDir()
	if dir == "" {
		return "", errors.New("cannot determine the data directory")
	}
	return dir, nil
}

func EnsureTldr() error {
	dir, err := tldrDir()
	if err != nil {
		return err
	}
	enPath := filepath.Join(dir, "en")
	if _, err := os.Stat(enPath); err == nil {
		return nil
	}
	return downloadTldr("en")
}

func downloadTldr(lang string) error {
	dir, err := tldrDir()
	if err != nil {
		return err
	}
	target := filepath.Join(dir, lang)
	os.MkdirAll(target, 0700)

	url := fmt.Sprintf("https://github.com/tldr-pages/tldr/releases/latest/download/tldr-pages.%s.zip", lang)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "tldr-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	io.Copy(tmp, resp.Body)

	archive, err := zip.OpenReader(tmp.Name())
	if err != nil {
		return err
	}
	defer archive.Close()

	for _, f := range archive.File {
		clean := filepath.Clean(f.Name)
		if strings.HasPrefix(clean, "..") {
			continue
		}
		if !strings.HasSuffix(clean, ".md") && !f.FileInfo().IsDir() {
			continue
		}
		dest := filepath.Join(target, clean)
		if f.FileInfo().IsDir() {
			os.MkdirAll(dest, 0700)
			continue
		}
		os.MkdirAll(filepath.Dir(dest), 0700)
		df, err := os.Create(dest)
		if err != nil {
			continue
		}
		sf, err := f.Open()
		if err != nil {
			df.Close()
			continue
		}
		io.Copy(df, sf)
		df.Close()
		sf.Close()
	}
	return nil
}

func GetTldrPage(command string) (string, error) {
	if !validPageName(command) {
		return "", fmt.Errorf("tldr page for '%s' not found", command)
	}
	if err := EnsureTldr(); err != nil {
		return "", err
	}
	dir, err := tldrDir()
	if err != nil {
		return "", err
	}

	platform := "linux"
	if runtime.GOOS == "darwin" {
		platform = "osx"
	}

	paths := []string{
		filepath.Join(dir, "en", platform, command+".md"),
		filepath.Join(dir, "en", "common", command+".md"),
	}
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("tldr page for '%s' not found", command)
}

func ListTldrPages() []string {
	dir, err := tldrDir()
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var names []string

	platform := "linux"
	if runtime.GOOS == "darwin" {
		platform = "osx"
	}

	for _, sub := range []string{platform, "common"} {
		entries, err := os.ReadDir(filepath.Join(dir, "en", sub))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}
