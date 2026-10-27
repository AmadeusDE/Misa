package webhost

import (
	"context"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/webui"
)

func TestAPIAdminMapsStoreRows(t *testing.T) {
	store, err := webui.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	_ = store.Update(func(st *webui.State) error {
		st.Devices = append(st.Devices, webui.Device{ID: "d1", Name: "phone", Created: now})
		st.Notices = append(st.Notices, webui.Notice{ID: "n1", Text: "new", At: now})
		return nil
	})
	a, ctx := APIAdmin(webui.NewAdmin(store, nil)), context.Background()
	if ds, err := a.WebDevices(ctx); err != nil || len(ds) != 1 || ds[0].ID != "d1" {
		t.Fatalf("devices %+v %v", ds, err)
	}
	if err := a.WebRevoke(ctx, "phone"); err != nil {
		t.Fatal(err)
	}
	if evs, _ := a.WebEvents(ctx, 5); len(evs) != 1 || evs[0].Kind != "revoke" {
		t.Fatalf("events %+v", evs)
	}
	if ns, _ := a.WebNotices(ctx); len(ns) != 1 {
		t.Fatalf("notices %+v", ns)
	}
	if err := a.WebDismissNotice(ctx, "n1"); err != nil {
		t.Fatal(err)
	}
	if ns, _ := a.WebNotices(ctx); len(ns) != 0 {
		t.Fatalf("notices %+v", ns)
	}
	if err := a.WebKillSession(ctx, "short"); err == nil {
		t.Fatal("short prefix accepted")
	}
}
