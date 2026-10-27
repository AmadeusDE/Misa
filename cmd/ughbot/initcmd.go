package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

func initCmd(args []string, out io.Writer) error {
	f := flag.NewFlagSet("init", flag.ContinueOnError)
	dir := f.String("d", config.DefaultConfigDir(), "config directory")
	if err := f.Parse(args); err != nil {
		return err
	}
	return writeExample(*dir, out)
}

// writeExample copies the embedded example into dir, leaving files that exist untouched.
func writeExample(dir string, out io.Writer) error {
	example := config.Example()
	return fs.WalkDir(example, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(name))
		if _, err := os.Stat(dst); err == nil {
			fmt.Fprintf(out, "kept     %s\n", dst)
			return nil
		}
		raw, err := fs.ReadFile(example, name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "created  %s\n", dst)
		return nil
	})
}
