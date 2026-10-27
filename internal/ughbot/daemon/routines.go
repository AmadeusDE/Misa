package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/sched"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

const (
	routineTick = 15 * time.Second
	// lateAfter is how overdue a fire must be to count as missed (the daemon was down).
	lateAfter = 2 * time.Minute
)

var _ api.Routines = (*Daemon)(nil)

func routineInfo(r store.Routine) api.RoutineInfo {
	return api.RoutineInfo{ID: r.ID, Owner: r.Owner, Bot: r.Bot, Prompt: r.Prompt, Cron: r.Cron, At: r.At, TZ: r.TZ,
		Missed: r.Missed, State: r.State, Next: r.Next, Last: r.Last, Fired: r.Fired, Created: r.Created}
}

func badRoutine(format string, a ...any) error {
	return fmt.Errorf("%w: %s", api.ErrBadRoutine, fmt.Sprintf(format, a...))
}

// resolve checks a request and turns it into a routine with its first fire time; a relative
// at time is fixed against now, so an approval stores an absolute time.
func (d *Daemon) resolve(req api.RoutineRequest, now time.Time) (store.Routine, error) {
	r := store.Routine{Owner: req.Owner, Bot: req.Bot, Prompt: strings.TrimSpace(req.Prompt),
		Cron: strings.TrimSpace(req.Cron), TZ: strings.TrimSpace(req.TZ), Missed: req.Missed}
	if r.Owner == "" {
		r.Owner = "user"
	}
	if _, ok := d.Lookup(r.Bot); !ok {
		return r, fmt.Errorf("%w: %q", api.ErrUnknownBot, r.Bot)
	}
	if r.Prompt == "" {
		return r, badRoutine("prompt is required")
	}
	switch r.Missed {
	case "":
		r.Missed = store.MissedOnce
	case store.MissedOnce, store.MissedSkip:
	default:
		return r, badRoutine("missed must be %q or %q", store.MissedOnce, store.MissedSkip)
	}
	spec := sched.Spec{Cron: r.Cron, TZ: r.TZ}
	if at := strings.TrimSpace(req.At); at != "" {
		loc, err := spec.Location()
		if err != nil {
			return r, badRoutine("%v", err)
		}
		if spec.At, err = sched.ParseAt(at, now, loc); err != nil {
			return r, badRoutine("%v", err)
		}
		r.At = spec.At
	}
	if err := spec.Check(); err != nil {
		return r, badRoutine("%v", err)
	}
	r.Next = spec.After(now)
	return r, nil
}

func specOf(r store.Routine) sched.Spec { return sched.Spec{Cron: r.Cron, At: r.At, TZ: r.TZ} }

func (d *Daemon) AddRoutine(_ context.Context, req api.RoutineRequest) (api.RoutineInfo, error) {
	r, err := d.resolve(req, time.Now())
	if err != nil {
		return api.RoutineInfo{}, err
	}
	return d.storeRoutine(r)
}

func (d *Daemon) storeRoutine(r store.Routine) (api.RoutineInfo, error) {
	id, err := d.st.AddRoutine(r)
	if err != nil {
		return api.RoutineInfo{}, err
	}
	d.wakeRoutines()
	saved, err := d.st.Routine(id)
	return routineInfo(saved), err
}

func (d *Daemon) Routines(_ context.Context, bot string) ([]api.RoutineInfo, error) {
	rows, err := d.st.Routines(bot)
	if err != nil {
		return nil, err
	}
	out := make([]api.RoutineInfo, len(rows))
	for i, r := range rows {
		out[i] = routineInfo(r)
	}
	return out, nil
}

func noRoutine(err error, id int64) error {
	if errors.Is(err, store.ErrNoRoutine) {
		return fmt.Errorf("%w: #%d", api.ErrNoRoutine, id)
	}
	return err
}

// PauseRoutine pauses or resumes; resuming continues from now, without firing what was missed.
func (d *Daemon) PauseRoutine(_ context.Context, id int64, paused bool) (api.RoutineInfo, error) {
	r, err := d.st.Routine(id)
	if err != nil {
		return api.RoutineInfo{}, noRoutine(err, id)
	}
	if r.State == store.RoutineDone {
		return api.RoutineInfo{}, badRoutine("#%d already fired and is done", id)
	}
	state, next := store.RoutinePaused, time.Time{}
	if !paused {
		state, next = store.RoutineActive, specOf(r).After(time.Now())
		if next.IsZero() {
			return api.RoutineInfo{}, badRoutine("#%d has no time left to fire", id)
		}
	}
	if err := d.st.SetRoutineState(id, state, next); err != nil {
		return api.RoutineInfo{}, noRoutine(err, id)
	}
	d.wakeRoutines()
	r, err = d.st.Routine(id)
	return routineInfo(r), err
}

func (d *Daemon) DeleteRoutine(_ context.Context, id int64) error {
	return noRoutine(d.st.DeleteRoutine(id), id)
}

func (d *Daemon) wakeRoutines() {
	select {
	case d.routineWake <- struct{}{}:
	default:
	}
}

func (d *Daemon) routineLoop(ctx context.Context) {
	t := time.NewTicker(routineTick)
	defer t.Stop()
	for {
		d.fireDue(time.Now())
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-d.routineWake:
		}
	}
}

// fireDue mails every due routine's bot once, however many fires were missed, and moves the
// routine to its next time after now. A routine whose missed policy is skip stays silent when
// it is overdue; one whose bot is gone is paused.
func (d *Daemon) fireDue(now time.Time) {
	due, err := d.st.DueRoutines(now)
	if err != nil {
		return
	}
	for _, r := range due {
		b, ok := d.Lookup(r.Bot)
		if !ok {
			_ = d.st.SetRoutineState(r.ID, store.RoutinePaused, time.Time{})
			continue
		}
		next := specOf(r).After(now)
		var mail *store.Mail
		if r.Missed != store.MissedSkip || now.Sub(r.Next) <= lateAfter {
			mail = &store.Mail{To: r.Bot, From: fmt.Sprintf("routine #%d", r.ID), Body: r.Prompt}
		}
		if ok, _, err := d.st.FireRoutine(r, next, mail, now); err == nil && ok && mail != nil {
			b.Notify()
		}
	}
}
