package webui

import (
	"fmt"
	"sync"
	"testing"
)

// Separate stores on one file stand in for the server and the admin CLI:
// concurrent updates must all land, none undone by a stale write.
func TestFileStoreUpdatesFromSeveralStoresAllLand(t *testing.T) {
	dir := t.TempDir()
	const stores, each = 4, 25
	var wg sync.WaitGroup
	for s := range stores {
		fs, err := NewFileStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				err := fs.Update(func(st *State) error {
					st.Devices = append(st.Devices, Device{ID: fmt.Sprintf("%d-%d", s, i)})
					return nil
				})
				if err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	fs, _ := NewFileStore(dir)
	n := 0
	fs.View(func(st *State) { n = len(st.Devices) })
	if n != stores*each {
		t.Errorf("%d devices, want %d (updates were lost)", n, stores*each)
	}
}
