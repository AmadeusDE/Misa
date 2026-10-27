package tui

import (
	"charm.land/bubbles/v2/key"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

type botAdminFake struct {
	*fakeBackend
	defs    map[string]api.BotDef
	saveErr error
}

func (f *botAdminFake) BotDef(_ context.Context, n string) (api.BotDef, error) { return f.defs[n], nil }
func (f *botAdminFake) CreateBot(_ context.Context, d api.BotDef) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.defs[d.Name] = d
	f.bots = append(f.bots, api.BotInfo{Name: d.Name, Model: d.Model, State: "idle", Editable: true})
	return nil
}
func (f *botAdminFake) UpdateBot(_ context.Context, d api.BotDef) error {
	f.defs[d.Name] = d
	return nil
}
func (f *botAdminFake) DeleteBot(_ context.Context, n string) error {
	delete(f.defs, n)
	f.bots = nil
	return nil
}

func TestCreateEditAndDeleteABotFromTheTUI(t *testing.T) {
	be := &botAdminFake{fakeBackend: newFake(), defs: map[string]api.BotDef{}}
	be.bots = nil
	m := ready(t, be, "")
	wantIn(t, m.View().Content, "no bots yet")

	press(m, "n")
	if m.form == nil {
		t.Fatal("n did not open the form")
	}
	typeText(m, "alpha")
	press(m, "tab")
	typeText(m, "fake/model")
	press(m, "tab")
	typeText(m, "be kind")
	press(m, "tab")
	press(m, "right") // files: read
	press(m, "tab")
	press(m, "space") // shell
	press(m, "tab")
	typeText(m, "2") // tasks
	m.form.focusOn(fDaily)
	typeText(m, "1.5")
	run(m, press(m, "ctrl+s"))
	if m.form != nil {
		t.Fatalf("form still open: %s", m.form.err)
	}
	got := be.defs["alpha"]
	want := config.Capabilities{Files: config.FilesRead, Shell: true, Tasks: 2}
	if got.Model != "fake/model" || got.PromptText != "be kind" || !reflect.DeepEqual(got.Capabilities, want) || got.Spend.Daily != 1.5 {
		t.Fatalf("saved %+v", got)
	}
	wantIn(t, m.View().Content, "alpha", "fake/model")

	run(m, press(m, "e"))
	if m.form == nil || !m.form.editing {
		t.Fatal("e did not open the edit form")
	}
	press(m, "esc")
	press(m, "x")
	if m.confirm != "alpha" {
		t.Fatal("x did not ask to confirm")
	}
	run(m, press(m, "y"))
	if _, ok := be.defs["alpha"]; ok {
		t.Fatal("bot not deleted")
	}
}

func TestFormShowsServerErrorsAndBadNumbers(t *testing.T) {
	be := &botAdminFake{fakeBackend: newFake(), defs: map[string]api.BotDef{}, saveErr: errors.New("bot already exists")}
	m := ready(t, be, "")
	press(m, "n")
	m.form.fields[fName].in.SetValue("x")
	m.form.fields[fModel].in.SetValue("m")
	m.form.fields[fDaily].in.SetValue("lots")
	press(m, "ctrl+s")
	wantIn(t, m.View().Content, "daily $ must be a number")
	m.form.fields[fDaily].in.SetValue("")
	run(m, press(m, "ctrl+s"))
	if m.form == nil {
		t.Fatal("form closed on error")
	}
	wantIn(t, m.View().Content, "bot already exists")
}

func TestTomlBotsCannotBeEdited(t *testing.T) {
	be := &botAdminFake{fakeBackend: newFake(), defs: map[string]api.BotDef{}}
	m := ready(t, be, "")
	press(m, "e")
	if m.form != nil {
		t.Fatal("opened an editor for a toml bot")
	}
	wantIn(t, m.View().Content, "ughbot.toml")
}

func TestFormInputsAvoidBrowserReserved(t *testing.T) {
	f := newBotForm(nil, 80, 24)
	for _, ff := range f.fields {
		var maps []reflect.Value
		switch ff.kind {
		case fieldText:
			maps = append(maps, reflect.ValueOf(ff.in.KeyMap))
		case fieldPrompt:
			maps = append(maps, reflect.ValueOf(ff.area.KeyMap))
		}
		for _, km := range maps {
			for i := 0; i < km.NumField(); i++ {
				if b, ok := km.Field(i).Interface().(key.Binding); ok {
					for _, k := range b.Keys() {
						if browserReserved[k] {
							t.Errorf("%s %s binds browser-reserved chord %q", ff.label, km.Type().Field(i).Name, k)
						}
					}
				}
			}
		}
	}
}

func TestPresetPrefillsTheFormButCreatesNothing(t *testing.T) {
	be := &botAdminFake{fakeBackend: newFake(), defs: map[string]api.BotDef{}}
	be.bots = nil
	m := ready(t, be, "")
	wantIn(t, m.View().Content, "press p to start from the bot-manager preset")
	press(m, "p")
	if m.form == nil || m.form.preset == "" {
		t.Fatal("p did not open the preset form")
	}
	wantIn(t, m.View().Content, "bot manager preset", "create bots")
	if len(be.defs) != 0 {
		t.Fatal("the preset created a bot by itself")
	}
	run(m, press(m, "ctrl+s"))
	if m.form == nil || len(be.defs) != 0 {
		t.Fatal("saved without a name and model")
	}
	wantIn(t, m.View().Content, "name is required")
	m.form.fields[fName].in.SetValue("boss")
	m.form.fields[fModel].in.SetValue("fake/m")
	run(m, press(m, "ctrl+s"))
	got, ok := be.defs["boss"]
	if !ok {
		t.Fatalf("not saved: %s", m.form.err)
	}
	if !got.Capabilities.CreateBots || got.Capabilities.Shell || got.Capabilities.Files != "" ||
		got.Spend.Daily != 1 || !strings.Contains(got.PromptText, "least it needs") {
		t.Fatalf("%+v", got)
	}
}
