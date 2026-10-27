package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/charmbracelet/x/ansi"
)

type routineFake struct {
	*approvalFake
	routines []api.RoutineInfo
	added    []api.RoutineRequest
}

func (f *routineFake) Routines(context.Context, string) ([]api.RoutineInfo, error) {
	return f.routines, nil
}
func (f *routineFake) AddRoutine(_ context.Context, req api.RoutineRequest) (api.RoutineInfo, error) {
	f.added = append(f.added, req)
	info := api.RoutineInfo{ID: int64(len(f.routines) + 1), Bot: req.Bot, Prompt: req.Prompt, Cron: req.Cron, State: "active",
		Missed: req.Missed, Owner: req.Owner, Next: time.Now().Add(time.Hour)}
	f.routines = append(f.routines, info)
	return info, nil
}
func (f *routineFake) PauseRoutine(_ context.Context, id int64, paused bool) (api.RoutineInfo, error) {
	for i := range f.routines {
		if f.routines[i].ID == id {
			f.routines[i].State = map[bool]string{true: "paused", false: "active"}[paused]
			return f.routines[i], nil
		}
	}
	return api.RoutineInfo{}, api.ErrNoRoutine
}
func (f *routineFake) DeleteRoutine(_ context.Context, id int64) error {
	for i := range f.routines {
		if f.routines[i].ID == id {
			f.routines = append(f.routines[:i], f.routines[i+1:]...)
			return nil
		}
	}
	return api.ErrNoRoutine
}

func TestRoutinesScreen(t *testing.T) {
	t.Setenv("WEBUI_DEVICE", "")
	be := &routineFake{approvalFake: newApprovalFake()}
	m := ready(t, be, "")
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	m.goTo(screenRoutines)
	run(m, m.fetchRoutines())
	wantIn(t, m.View().Content, "no routines yet")

	press(m, "n")
	if m.routines.form == nil || m.routines.form.fields[rBot].in.Value() != "a" {
		t.Fatal("n should open the form for the selected bot")
	}
	typeText(m, "61 * * * *")
	wantIn(t, m.View().Content, "is not in 0-59")
	m.routines.form.fields[rWhen].in.SetValue("0 9 * * mon")
	wantIn(t, m.View().Content, "first fires Mon")
	m.routines.form.focusOn(rPrompt)
	typeText(m, "weekly review")
	run(m, press(m, "ctrl+s"))
	if m.routines.form != nil || len(be.added) != 1 {
		t.Fatalf("not saved: %+v", m.routines.form)
	}
	if r := be.added[0]; r.Bot != "a" || r.Cron != "0 9 * * mon" || r.Prompt != "weekly review" || r.Missed != "once" || r.Owner != "tui" {
		t.Fatalf("%+v", r)
	}
	run(m, m.fetchRoutines())
	wantIn(t, m.View().Content, "#1", "active", "0 9 * * mon", "weekly review", "routine #1 added")

	run(m, press(m, "space"))
	if be.routines[0].State != "paused" {
		t.Fatal("space did not pause")
	}
	press(m, "x")
	wantIn(t, m.View().Content, "delete this routine? y/N")
	run(m, press(m, "y"))
	if len(be.routines) != 0 {
		t.Fatal("not deleted")
	}
}

func TestRoutineFormAtTimeAndMissedPolicy(t *testing.T) {
	be := &routineFake{approvalFake: newApprovalFake()}
	m := ready(t, be, "")
	m.goTo(screenRoutines)
	run(m, m.fetchRoutines())
	press(m, "n")
	f := m.routines.form
	f.focusOn(rKind)
	press(m, "right") // at
	f.fields[rWhen].in.SetValue("in 2h")
	f.focusOn(rMissed)
	press(m, "right") // skip
	f.fields[rPrompt].area.SetValue("stretch")
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "first fires") {
		t.Fatalf("no preview:\n%s", view)
	}
	run(m, press(m, "ctrl+s"))
	if len(be.added) != 1 || be.added[0].At != "in 2h" || be.added[0].Cron != "" || be.added[0].Missed != "skip" {
		t.Fatalf("%+v", be.added)
	}
}
