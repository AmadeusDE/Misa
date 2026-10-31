package builtins

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSplitAdditionalSuffix(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantErr  bool
		wantFile string
	}{
		{
			name:     "valid suffix",
			args:     []string{"--lines=2", "--additional-suffix=.txt", "-"},
			wantErr:  false,
			wantFile: "xaa.txt",
		},
		{
			name:    "suffix with directory separator",
			args:    []string{"--lines=2", "--additional-suffix=a/b", "-"},
			wantErr: true,
		},
		{
			name:    "suffix with trailing slash",
			args:    []string{"--lines=2", "--additional-suffix=a/", "-"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := t.TempDir()
			ctx := &Context{
				Dir:    tmpdir,
				Stdin:  bytes.NewReader([]byte("1\n2\n3\n4\n")),
				Stdout: io.Discard,
				Stderr: &bytes.Buffer{},
				Args:   tt.args,
			}

			rc := runSplit(ctx)
			if tt.wantErr {
				if rc == 0 {
					t.Errorf("expected error, got success")
				}
			} else {
				if rc != 0 {
					t.Errorf("expected success, got error")
				}
				if tt.wantFile != "" {
					path := filepath.Join(tmpdir, tt.wantFile)
					if _, err := os.Stat(path); err != nil {
						t.Errorf("expected file %s to exist", tt.wantFile)
					}
				}
			}
		})
	}
}

func TestSplitRecordSeparator(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "single separator",
			args:    []string{"-t", ":", "-l1", "-"},
			wantErr: false,
		},
		{
			name:    "same separator twice",
			args:    []string{"-t", ":", "-t", ":", "-l1", "-"},
			wantErr: false,
		},
		{
			name:    "different separators",
			args:    []string{"-t", ":", "-t", "a", "-"},
			wantErr: true,
		},
		{
			name:    "multi-char separator",
			args:    []string{"-t", "ab", "-"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := t.TempDir()
			ctx := &Context{
				Dir:    tmpdir,
				Stdin:  bytes.NewReader([]byte("1:2:3\n")),
				Stdout: io.Discard,
				Stderr: &bytes.Buffer{},
				Args:   tt.args,
			}

			rc := runSplit(ctx)
			if tt.wantErr {
				if rc == 0 {
					t.Errorf("expected error, got success")
				}
			} else {
				if rc != 0 {
					t.Errorf("expected success, got error: %s", ctx.Stderr.(*bytes.Buffer).String())
				}
			}
		})
	}
}

func TestSplitGuardInput(t *testing.T) {
	t.Run("direct file overwrite", func(t *testing.T) {
		tmpdir := t.TempDir()
		inputFile := filepath.Join(tmpdir, "xaa")
		os.WriteFile(inputFile, []byte("1\n2\n3\n4\n5\n6\n"), 0644)

		stderr := &bytes.Buffer{}
		ctx := &Context{
			Dir:    tmpdir,
			Stdin:  nil,
			Stdout: io.Discard,
			Stderr: stderr,
			Args:   []string{"-C", "6", inputFile},
		}

		rc := runSplit(ctx)
		if rc == 0 {
			t.Errorf("expected error when overwriting input file")
		}
		if !bytes.Contains(stderr.Bytes(), []byte("would overwrite input")) {
			t.Errorf("expected 'would overwrite input' error message, got: %s", stderr.String())
		}
	})

	t.Run("stdin redirected from file", func(t *testing.T) {
		tmpdir := t.TempDir()
		inputFile := filepath.Join(tmpdir, "xaa")
		os.WriteFile(inputFile, []byte("1\n2\n3\n4\n5\n6\n"), 0644)

		inputFh, _ := os.Open(inputFile)
		defer inputFh.Close()

		stderr := &bytes.Buffer{}
		ctx := &Context{
			Dir:    tmpdir,
			Stdin:  inputFh,
			Stdout: io.Discard,
			Stderr: stderr,
			Args:   []string{"-C", "6", "-"},
		}

		rc := runSplit(ctx)
		if rc == 0 {
			t.Errorf("expected error when stdin redirected from file being overwritten")
		}
		if !bytes.Contains(stderr.Bytes(), []byte("would overwrite input")) {
			t.Errorf("expected 'would overwrite input' error message, got: %s", stderr.String())
		}
	})
}

func TestSplitWriteError(t *testing.T) {
	t.Run("write error handling", func(t *testing.T) {
		skipOnWindows(t, "a Windows directory cannot be made unwritable with a mode")
		tmpdir := t.TempDir()

		// Create a read-only directory to cause write error
		readOnlyDir := filepath.Join(tmpdir, "readonly")
		os.Mkdir(readOnlyDir, 0444)
		defer os.Chmod(readOnlyDir, 0755)

		stderr := &bytes.Buffer{}
		ctx := &Context{
			Dir:    readOnlyDir,
			Stdin:  bytes.NewReader([]byte("data\n")),
			Stdout: io.Discard,
			Stderr: stderr,
			Args:   []string{"-b", "1", "-"},
		}

		rc := runSplit(ctx)
		// Should fail due to write error
		if rc == 0 {
			t.Errorf("expected error due to write failure")
		}
	})
}
