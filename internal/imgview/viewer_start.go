package imgview

import (
	"errors"
	"fmt"
	"path/filepath"
)

// SetStartPath selects an existing item before Init, without starting decoding
// or changing collection order. Paths are compared lexically, not by inode.
func (m *Viewer) SetStartPath(path string) error {
	if path == "" {
		return errors.New("start file must not be empty")
	}
	path = filepath.Clean(path)
	for i, item := range m.items {
		if filepath.Clean(item.path) == path {
			m.selectIndex(i)
			return nil
		}
	}
	return fmt.Errorf("start file %q is not in the image collection", path)
}
