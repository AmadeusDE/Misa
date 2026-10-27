package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDecodeStoredConvertsLegacy(t *testing.T) {
	raw := `{"Name":"a","Model":"m","PromptText":"p","Tools":["read","bash","start_task"],"CompactAt":9,
		"Spend":{"Daily":1,"Monthly":2},"Grants":{"Message":["*"],"Manage":["b"],"Library":false,"Hosts":null,"Memory":false},"Workdir":""}`
	b, err := DecodeStored([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := Capabilities{Files: FilesRead, Shell: true, Tasks: 3, Message: []string{"*"}, Manage: []string{"b"}}
	if b.Model != "m" || b.PromptText != "p" || b.CompactAt != 9 || b.Spend.Monthly != 2 || !reflect.DeepEqual(b.Capabilities, want) {
		t.Fatalf("%+v", b)
	}
	if b, _ := DecodeStored([]byte(`{"Model":"m","Tools":["edit"]}`)); b.Capabilities.Files != FilesWrite {
		t.Fatalf("edit means write: %+v", b.Capabilities)
	}
}

func TestDecodeStoredRoundTrip(t *testing.T) {
	in := Bot{Name: "a", Model: "m", Capabilities: Capabilities{Files: FilesWrite, Tasks: 1, CreateBots: true}}
	raw, _ := json.Marshal(in)
	out, err := DecodeStored(raw)
	if err != nil || !reflect.DeepEqual(in, out) {
		t.Fatalf("%v\n%+v\n%+v", err, in, out)
	}
}
