package webadmin

import (
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/webui"
)

func TestTUIAdapterMapsAndActsOnStore(t *testing.T) {
	store, err := webui.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	_ = store.Update(func(st *webui.State) error {
		st.Devices = append(st.Devices, webui.Device{ID: "d1", Name: "phone", Created: now})
		st.Notices = append(st.Notices, webui.Notice{ID: "n1", Text: "hello", At: now})
		return nil
	})
	a := TUI(webui.NewAdmin(store, nil))
	ds, err := a.Devices()
	if err != nil || len(ds) != 1 || ds[0].Name != "phone" {
		t.Fatalf("devices = %+v %v", ds, err)
	}
	if err := a.Rename("phone", "tablet"); err != nil {
		t.Fatal(err)
	}
	if err := a.Revoke("tablet"); err != nil {
		t.Fatal(err)
	}
	ds, _ = a.Devices()
	if ds[0].Name != "tablet" || !ds[0].Revoked {
		t.Fatalf("after = %+v", ds)
	}
	evs, _ := a.Events(10)
	if len(evs) != 1 || evs[0].Kind != "revoke" {
		t.Fatalf("events = %+v", evs)
	}
	if ns, _ := a.Notices(); len(ns) != 1 || ns[0].Text != "hello" {
		t.Fatalf("notices = %+v", ns)
	}
	if err := a.DismissNotice("n1"); err != nil {
		t.Fatal(err)
	}
	if ns, _ := a.Notices(); len(ns) != 0 {
		t.Fatalf("notices = %+v", ns)
	}
	if ss, err := a.Sessions(); err != nil || len(ss) != 0 {
		t.Fatalf("sessions = %+v %v", ss, err)
	}
}
