package vi

import (
	"os"
	"path/filepath"
)

func writeFilePreservingLinks(path string, data []byte, info os.FileInfo) error {
	link, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if link != nil && link.Mode()&os.ModeSymlink != 0 || info != nil && mustWriteInPlace(info) {
		return writeInPlace(path, data, info)
	}
	if info == nil {
		return os.WriteFile(path, data, 0644)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".misa-vi-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err == nil {
		err = f.Chmod(info.Mode())
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func writeInPlace(path string, data []byte, info os.FileInfo) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil && info != nil {
		var current os.FileInfo
		current, err = f.Stat()
		if err == nil && current.Mode() != info.Mode() {
			err = f.Chmod(info.Mode())
		}
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}
