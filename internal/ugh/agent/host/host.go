// Package host assembles a runnable root agent for one cwd: provider, permission
// gate, tool registry and a resumable session, all under explicit config and
// state roots. `ugh -p` and embedding programs (ughbot) share it.
package host

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/admission"
	"github.com/AmadeusDE/Misa/internal/ugh/agent"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"github.com/AmadeusDE/Misa/internal/ugh/provider"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"
	"github.com/AmadeusDE/Misa/internal/ugh/session"
	"github.com/AmadeusDE/Misa/internal/ugh/tools"
)

type Options struct {
	Cwd      string
	Model    string // provider/model; a resumed session keeps its own
	Thinking string
	System   string // "" is agent.DefaultSystem
	Mode     string // manual | classify | auto; "" follows config
	MaxTurns int    // 0 follows config agent.max_turns_root

	// Roots isolates config and state; the zero value is the user's ugh.
	Roots roots.Roots

	// Resume continues the latest root session of Cwd; ResumePath names one.
	Resume     bool
	ResumePath string
	// OverrideResumed makes Model and System win over what the resumed session
	// recorded, for callers (daemons) whose config is the source of truth.
	OverrideResumed bool

	// Tools defaults to the built-in toolset.
	Tools *tools.Registry
	// Prompter answers permission prompts. It is required unless Gate is set.
	// A prompter with SetHeadless(string) is told the policy's unattended answer.
	Prompter perm.Prompter
	// Gate replaces the permission engine built from Roots (tests, custom policy).
	Gate perm.Gate
	// Provider replaces the resolved client for Model.
	Provider llm.Provider

	// Out receives assistant text; Trace the tool trace and notices.
	Out, Trace io.Writer

	// Configure edits the agent configuration just before construction.
	Configure func(*agent.Config)
}

// Runner is one constructed root agent and the session it appends to.
type Runner struct {
	Agent   *agent.Agent
	Session *session.Store
	Runtime *agent.Runtime
	Gate    perm.Gate
	Roots   roots.Roots

	Model, Thinking, System string
	// Resumed is the number of history messages loaded from the session.
	Resumed int

	councils []session.Record
}

func New(ctx context.Context, o Options) (*Runner, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if o.Cwd == "" {
		return nil, errors.New("host: Cwd required")
	}
	if o.Gate == nil && o.Prompter == nil {
		return nil, errors.New("host: Prompter or Gate required")
	}
	if o.Trace == nil {
		o.Trace = io.Discard
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	if o.System == "" {
		o.System = agent.DefaultSystem
	}
	rt := agent.NewRuntimeIn(o.Roots)
	r := &Runner{Runtime: rt, Roots: o.Roots, Model: o.Model, Thinking: o.Thinking, System: o.System}
	var hist []llm.Message
	var archive []session.Record
	resumePath := o.ResumePath
	if resumePath == "" && o.Resume {
		p, err := session.LatestIn(o.Roots, o.Cwd)
		if err != nil {
			return nil, err
		}
		resumePath = p
	}
	if resumePath != "" {
		loaded, err := session.Load(resumePath)
		if err != nil {
			return nil, err
		}
		hist, r.councils, archive = loaded.Messages, loaded.Councils, loaded.Display
		if loaded.Meta.Run != "" {
			rt.RunID = loaded.Meta.Run // children of the resumed run stay linked
		}
		if loaded.Meta.Model == "" {
			return nil, fmt.Errorf("session %s has no meta header", resumePath)
		}
		if !o.OverrideResumed || o.Model == "" {
			r.Model = loaded.Meta.Model
		}
		if loaded.Meta.Thinking != "" && r.Thinking == "" {
			r.Thinking = loaded.Meta.Thinking
		}
		if r.Thinking == "" {
			r.Thinking = agent.DefaultThinking
		}
		if loaded.Meta.System != "" && !(o.OverrideResumed && o.System != "") {
			r.System = loaded.Meta.System
		}
		if r.Session, err = session.Open(resumePath); err != nil {
			return nil, err
		}
		r.Resumed = len(hist)
		fmt.Fprintf(o.Trace, "[resumed %d messages from %s]\n", len(hist), resumePath)
	} else {
		if r.Thinking == "" {
			r.Thinking = agent.DefaultThinking
		}
		sess, err := session.CreateIn(o.Roots, o.Cwd, session.Record{Model: r.Model, Thinking: r.Thinking, System: r.System, Cwd: o.Cwd, Run: rt.RunID})
		if err != nil {
			return nil, err
		}
		r.Session = sess
	}
	if err := r.build(o, hist, archive); err != nil {
		r.Session.Close()
		return nil, err
	}
	return r, nil
}

func (r *Runner) build(o Options, hist []llm.Message, archive []session.Record) error {
	if r.Thinking != "" {
		if t := agent.ClampThinking(r.Model, r.Thinking); t != r.Thinking {
			fmt.Fprintf(o.Trace, "[thinking %s → %s: closest level %s supports]\n", r.Thinking, t, r.Model)
			r.Thinking = t
		}
	}
	prov, model := o.Provider, modelOf(r.Model)
	if prov == nil {
		var err error
		if prov, model, err = provider.ResolveIn(o.Roots, r.Model); err != nil {
			return err
		}
	}
	r.Gate = o.Gate
	if r.Gate == nil {
		eng, err := provider.GateIn(o.Roots, o.Mode, o.Prompter)
		if err != nil {
			return err
		}
		if h, ok := o.Prompter.(interface{ SetHeadless(string) }); ok {
			h.SetHeadless(eng.Policy.HeadlessPrompt) // config decides what an unattended prompt becomes
		}
		r.Gate = eng
	}
	reg := o.Tools
	if reg == nil {
		reg = tools.NewRegistry(tools.All()...)
	}
	rt, sess := r.Runtime, r.Session
	maxTurns := o.MaxTurns
	if maxTurns <= 0 {
		maxTurns = rt.Config.Agent.MaxTurnsRoot
	}
	inbox, _ := session.InboxPathIn(o.Roots, rt.RunID, "root")
	cfg := agent.Config{
		Roots:    o.Roots,
		MaxTurns: maxTurns,
		Provider: prov, Model: model, System: r.System, Thinking: r.Thinking,
		Tools: reg, Cwd: o.Cwd, Gate: r.Gate,
		Out: o.Out, Trace: o.Trace, History: hist,
		Tree: rt.Tree, Spawn: rt.Spawn, StartChild: rt.StartChild, Resume: rt.ResumeChild, StreamLimit: rt.Limit, Runtime: rt,
		OnMessage:    func(m llm.Message) { _ = sess.Append(session.Record{Type: "message", Message: &m}) },
		OnCompact:    func(h []llm.Message) { _ = sess.Append(session.Record{Type: "compaction", Seed: h}) },
		OnRecord:     func(rec session.Record) { _ = sess.Append(rec) },
		OnStatus:     func(st string) { _ = sess.Append(session.Record{Type: "status", Status: st}) },
		AutoCompact:  rt.Config.Agent.AutoCompact,
		Inbox:        inbox,
		ToolTimeout:  time.Duration(rt.Config.Tools.TimeoutS) * time.Second,
		ProviderName: providerOf(r.Model), Admission: agent.DefaultAdmission(admission.PriorityRoot, "root"),
		Scratch: session.ScratchPathIn(o.Roots, rt.RunID, "root"),
		Archive: archive, BlobDir: session.BlobDir(sess.Path()),
		Context: agent.ContextConfigIn(o.Roots, rt.Config),
	}
	if o.Configure != nil {
		o.Configure(&cfg)
	}
	r.Agent = agent.New(cfg)
	for _, rec := range r.councils {
		r.Agent.RestoreCouncil(rec) // outcomes replay; approvals never do
	}
	return nil
}

// Run sends one prompt and appends its result record to the session. The
// record's status is ok, turn-out, cancelled or error.
func (r *Runner) Run(ctx context.Context, prompt string) (string, error) {
	final, err := r.Agent.Run(ctx, prompt)
	rec := session.Record{Type: "result", Run: r.Runtime.RunID, Agent: "root", Model: r.Model,
		Status: "ok", Turns: r.Agent.Turns, Usage: &r.Agent.Usage}
	switch {
	case err == nil:
	case errors.Is(err, agent.ErrTurnOut):
		rec.Status = "turn-out"
	case ctx.Err() != nil:
		rec.Status = "cancelled"
	default:
		rec.Status = "error"
	}
	if err != nil {
		rec.Error = err.Error()
	}
	_ = r.Session.Append(rec)
	return final, err
}

// WaitBackground gives a gate that classifies in the background a moment to
// land its decisions before the process exits.
func (r *Runner) WaitBackground(d time.Duration) {
	if w, ok := r.Gate.(interface{ WaitBackground(time.Duration) }); ok {
		w.WaitBackground(d)
	}
}

func (r *Runner) Close() error { return r.Session.Close() }

func providerOf(ref string) string { p, _, _ := strings.Cut(ref, "/"); return p }

func modelOf(ref string) string {
	if _, m, ok := strings.Cut(ref, "/"); ok {
		return m
	}
	return ref
}
