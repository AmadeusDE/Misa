package perm

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeCl is a scripted classifier that counts its runs.
type fakeCl struct {
	name    string
	flagged bool
	reason  string
	err     error
	runs    int32 // atomic: auto mode classifies on another goroutine
}

func (f *fakeCl) Name() string { return f.name }
func (f *fakeCl) Classify(_ context.Context, _ Call) Verdict {
	atomic.AddInt32(&f.runs, 1)
	return Verdict{Classifier: f.name, Flagged: f.flagged, Reason: f.reason, Err: f.err}
}

func engineFor(t *testing.T, p Policy, in io.Reader, interactive bool, cls ...Classifier) (*Engine, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "decisions.jsonl")
	log, err := NewLog(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	e := NewEngine(p, NewBroker(in, io.Discard, interactive, p.HeadlessPrompt), log)
	e.Add(cls...)
	return e, path
}

func TestEngineSoloStopSkipsModelClassifiers(t *testing.T) {
	norm := &fakeCl{name: "static_normal", flagged: true, reason: "recursive rm on /"}
	model := &fakeCl{name: "jev_or"}
	e, path := engineFor(t, pol("classify", "any", "max"), nil, false, norm, model)
	oc := e.Check(context.Background(), Call{Tool: "bash", Command: "rm -rf /"})
	if oc.Action != Stop {
		t.Fatalf("outcome = %+v, want stop", oc)
	}
	if atomic.LoadInt32(&model.runs) != 0 {
		t.Errorf("model classifier ran %d times after a solo stop, want 0", model.runs)
	}
	waitForFile(t, path, `"action":"stop"`)
}

func TestEngineHeadlessPromptDenies(t *testing.T) {
	ext := &fakeCl{name: "static_extreme", flagged: true, reason: "rm with recursive/force flags"}
	e, path := engineFor(t, pol("classify", "any", "max"), nil, false, ext)
	oc := e.Check(context.Background(), Call{Tool: "bash", Command: "rm -rf ./build"})
	if oc.Action != Deny {
		t.Fatalf("outcome = %+v, want deny", oc)
	}
	if !strings.Contains(oc.Reason, "no human attached") {
		t.Errorf("reason = %q, want the headless wording", oc.Reason)
	}
	waitForFile(t, path, `"answer":"n"`)
}

func TestEngineInteractiveAnswers(t *testing.T) {
	for in, want := range map[string]string{
		"y\n": Allow, "n\n": Deny, "s\n": Stop,
	} {
		ext := &fakeCl{name: "static_extreme", flagged: true, reason: "rm -f"}
		e, _ := engineFor(t, pol("classify", "any", "max"), strings.NewReader(in), true, ext)
		oc := e.Check(context.Background(), Call{Tool: "bash", Command: "rm -rf ./build"})
		if oc.Action != want {
			t.Errorf("stdin %q → %q (%s), want %q", in, oc.Action, oc.Reason, want)
		}
		if oc.Action == Deny && oc.Reason != "denied by user" {
			t.Errorf("deny reason = %q, want \"denied by user\"", oc.Reason)
		}
	}
}

// Manual mode asks about every call and classifies nothing: no triage, no
// static rules, no model classifiers, no output check.
func TestEngineManualModeAsksWithoutClassifying(t *testing.T) {
	norm := &fakeCl{name: "static_normal", flagged: true, reason: "recursive rm on /"}
	ext := &fakeCl{name: "static_extreme"}
	jev := &fakeCl{name: "jev_or"}
	p := pol("manual", "any", "max")
	p.Classifiers["jev_or"] = ClassifierConfig{State: "on", OnFlag: "prompt", Tools: []string{"*"}}
	for in, want := range map[string]string{"y\n": Allow, "n\n": Deny} {
		e, _ := engineFor(t, p, strings.NewReader(in), true, norm, ext, jev)
		triaged := false
		e.Triage = func(Call) (string, string) { triaged = true; return "trivial", "test" }
		judge := &fakeJudge{}
		e.Output = judge
		e.Policy.Classifiers[OutputCheckName] = ClassifierConfig{State: "on"}
		oc := e.Check(context.Background(), Call{Tool: "bash", Command: "rm -rf /", Agent: "root"})
		e.CheckOutput(context.Background(), "root", "bash", "output")
		if oc.Action != want {
			t.Errorf("answer %q → %+v, want %s", in, oc, want)
		}
		if triaged || atomic.LoadInt32(&norm.runs)+atomic.LoadInt32(&ext.runs)+atomic.LoadInt32(&jev.runs)+atomic.LoadInt32(&judge.calls) != 0 {
			t.Errorf("manual mode classified: triage=%v static=%d/%d jev=%d output=%d", triaged, norm.runs, ext.runs, jev.runs, judge.calls)
		}
	}
	// Even a read is asked about; with nobody to answer it is denied.
	e, _ := engineFor(t, pol("manual", "any", "max"), nil, false)
	if oc := e.Check(context.Background(), Call{Tool: "read", Args: []byte(`{"path":"a.txt"}`)}); oc.Action != Deny {
		t.Fatalf("manual + headless = %+v, want deny", oc)
	}
}

func TestEngineSessionAllowSkipsRepeatPrompt(t *testing.T) {
	ext := &fakeCl{name: "static_extreme", flagged: true, reason: "rm -f"}
	e, _ := engineFor(t, pol("classify", "any", "max"), strings.NewReader("a\n"), true, ext)
	call := Call{Tool: "bash", Command: "rm -rf ./build"}
	if oc := e.Check(context.Background(), call); oc.Action != Allow {
		t.Fatalf("first = %+v, want allow", oc)
	}
	// The second check must not prompt again (stdin is exhausted → deny).
	if oc := e.Check(context.Background(), call); oc.Action != Allow || !strings.Contains(oc.Reason, "session") {
		t.Fatalf("second = %+v, want session allow", oc)
	}
}

func TestEngineAutoModeAllowsAndLogsInBackground(t *testing.T) {
	norm := &fakeCl{name: "static_normal", flagged: true, reason: "recursive rm on /"}
	e, path := engineFor(t, pol("auto", "any", "max"), nil, false, norm)
	start := time.Now()
	oc := e.Check(context.Background(), Call{Tool: "bash", Command: "rm -rf /"})
	if oc.Action != Allow {
		t.Fatalf("auto outcome = %+v, want allow", oc)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Errorf("auto mode waited %s, must not block", time.Since(start))
	}
	waitForFile(t, path, `"action":"allow"`)
	waitForFile(t, path, `"mode":"auto"`)
	if atomic.LoadInt32(&norm.runs) == 0 {
		t.Error("auto mode must still run classifiers in the background")
	}
}

func TestEngineClassifierErrorUsesOnError(t *testing.T) {
	p := pol("classify", "any", "max")
	c := p.Classifiers["jev_or"]
	c.OnError = "stop"
	p.Classifiers["jev_or"] = c
	bad := &fakeCl{name: "jev_or", err: context.DeadlineExceeded}
	e, _ := engineFor(t, p, nil, false, bad)
	if oc := e.Check(context.Background(), Call{Tool: "bash", Command: "ls"}); oc.Action != Stop {
		t.Fatalf("on_error=stop outcome = %+v, want stop", oc)
	}
	// Default on_error=ignore lets the run continue while another model judges.
	p2 := pol("classify", "any", "max")
	e2, _ := engineFor(t, p2, nil, false, &fakeCl{name: "jev_or", err: context.DeadlineExceeded}, &fakeCl{name: "clef_or"})
	if oc := e2.Check(context.Background(), Call{Tool: "bash", Command: "ls"}); oc.Action != Allow {
		t.Fatalf("on_error=ignore outcome = %+v, want allow", oc)
	}
	// With no model verdict at all, a headless run is refused, not waved through.
	e3, _ := engineFor(t, p2, nil, false, &fakeCl{name: "jev_or", err: context.DeadlineExceeded}, &fakeCl{name: "clef_or", err: context.DeadlineExceeded})
	if oc := e3.Check(context.Background(), Call{Tool: "bash", Command: "ls"}); oc.Action != Deny {
		t.Fatalf("all models failed outcome = %+v, want deny (headless prompt)", oc)
	}
}

func TestEngineOffClassifierDoesNotRun(t *testing.T) {
	p := pol("classify", "any", "max")
	c := p.Classifiers["jev_or"]
	c.State = "off"
	p.Classifiers["jev_or"] = c
	off := &fakeCl{name: "jev_or", flagged: true}
	e, _ := engineFor(t, p, nil, false, off)
	if oc := e.Check(context.Background(), Call{Tool: "bash", Command: "ls"}); oc.Action != Allow {
		t.Fatalf("outcome = %+v, want allow", oc)
	}
	if atomic.LoadInt32(&off.runs) != 0 {
		t.Errorf("off classifier ran %d times", off.runs)
	}
}

// waitForFile polls the decision log until it contains want (auto mode logs
// from a goroutine) or fails the test.
func waitForFile(t *testing.T, path, want string) {
	t.Helper()
	for i := 0; i < 2000; i++ {
		if b, err := os.ReadFile(path); err == nil && strings.Contains(string(b), want) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	b, _ := os.ReadFile(path)
	t.Fatalf("decision log never contained %q; got:\n%s", want, b)
}

// Plain auto mode applies nothing, not even static stops: it is the
// "ignore every classifier" mode.
func TestEngineAutoDoesNotApplyStaticStops(t *testing.T) {
	norm := &fakeCl{name: "static_normal", flagged: true, reason: "recursive rm on ~"}
	e, _ := engineFor(t, pol("auto", "any", "max"), nil, false, norm)
	if oc := e.Check(context.Background(), Call{Tool: "bash", Command: "rm -rf ~"}); oc.Action != Allow {
		t.Fatalf("auto = %+v, want allow", oc)
	}
	e.WaitBackground(time.Second)
}

// slowCl is a fakeCl that takes d to answer, like a network classifier.
type slowCl struct {
	fakeCl
	d time.Duration
}

func (s *slowCl) Classify(ctx context.Context, c Call) Verdict {
	time.Sleep(s.d)
	return s.fakeCl.Classify(ctx, c)
}

func TestEngineTierPanels(t *testing.T) {
	mk := func() map[string]*fakeCl {
		return map[string]*fakeCl{"jev_or": {name: "jev_or"}, "clef_or": {name: "clef_or"}, "clef_flash_or": {name: "clef_flash_or"}, "luna_or": {name: "luna_or"}}
	}
	for _, c := range []struct {
		tier    string
		extreme bool
		want    string // classifiers that must run, sorted
	}{
		{"trivial", false, ""},
		{"low", false, "jev_or"},
		{"middle", false, "jev_or"},
		{"high", false, "jev_or"},
		{"trivial", true, "jev_or"}, // a static flag lifts the call to high
	} {
		models := mk()
		p := pol("classify", "any", "max")
		for name := range models {
			p.Classifiers[name] = ClassifierConfig{State: "on", OnFlag: "prompt", Tools: []string{"*"}, TimeoutMS: 3000}
		}
		p.Tiers = DefaultTiers()
		cls := []Classifier{&fakeCl{name: "static_normal"}, &fakeCl{name: "static_extreme", flagged: c.extreme, reason: "rm with recursive/force flags"}}
		for _, m := range models {
			cls = append(cls, m)
		}
		e, path := engineFor(t, p, strings.NewReader("y\n"), true, cls...)
		e.Triage = func(Call) (string, string) { return c.tier, "test" }
		e.Check(context.Background(), Call{Tool: "bash", Command: "x", Agent: "root"})
		var ran []string
		for name, m := range models {
			if atomic.LoadInt32(&m.runs) > 0 {
				ran = append(ran, name)
			}
		}
		sort.Strings(ran)
		if got := strings.Join(ran, ","); got != c.want {
			t.Errorf("tier %s (extreme=%v): ran %q, want %q", c.tier, c.extreme, got, c.want)
		}
		wantTier := c.tier
		if c.extreme {
			wantTier = "high"
		}
		waitForFile(t, path, `"tier":"`+wantTier+`"`)
	}
}

// Classify mode asks only when static rules or Jev flag the call.
func TestEngineClassifyAsksOnlyOnFlags(t *testing.T) {
	for _, c := range []struct {
		name          string
		static, model bool
		want          string
	}{
		{"clean", false, false, Allow},
		{"static flag", true, false, Deny},
		{"jev flag", false, true, Deny},
	} {
		p := pol("classify", "any", "max")
		p.Classifiers["jev_or"] = ClassifierConfig{State: "on", OnFlag: "prompt", Tools: []string{"*"}}
		p.Tiers = DefaultTiers()
		ext := &fakeCl{name: "static_extreme", flagged: c.static, reason: "rm -f"}
		jev := &fakeCl{name: "jev_or", flagged: c.model, reason: "irreversible 0.9"}
		// "n" answers any prompt: a deny means the call was asked about.
		e, _ := engineFor(t, p, strings.NewReader("n\n"), true, &fakeCl{name: "static_normal"}, ext, jev)
		e.Triage = func(Call) (string, string) { return "low", "test" }
		if oc := e.Check(context.Background(), Call{Tool: "bash", Command: "x", Agent: "root"}); oc.Action != c.want {
			t.Errorf("%s: %+v, want %s", c.name, oc, c.want)
		}
	}
}
