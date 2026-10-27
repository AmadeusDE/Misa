// Package agent implements the M1 tool loop: stream one assistant turn,
// run the tools it asks for, feed the results back, and repeat until the
// model answers without calling a tool (or ctx is cancelled).
//
// M2 adds the permission gate: every tool call is checked first; a denial
// becomes tool feedback for the model, a stop halts the whole run.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AmadeusDE/Misa/internal/ugh/roots"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/llm/chat"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"github.com/AmadeusDE/Misa/internal/ugh/perm/model"
	"github.com/AmadeusDE/Misa/internal/ugh/provider"
	"github.com/AmadeusDE/Misa/internal/ugh/provstate"
	"github.com/AmadeusDE/Misa/internal/ugh/session"
	"github.com/AmadeusDE/Misa/internal/ugh/tools"
	"github.com/AmadeusDE/Misa/internal/ugh/usage"
)

// Event is one progress notification for UIs and loggers (PLAN.md §7: all
// provider/tool work happens in goroutines; UIs receive these as tea.Msgs).
type Event struct {
	Kind    string // text | thinking | tool_args | tool_start | tool_done | usage | note | tool_progress
	Text    string
	Call    llm.ToolCall // tool_start / tool_done
	Output  string       // tool_done: the tool result
	Err     error        // tool_done: the tool error, if any
	Elapsed time.Duration
	Usage   llm.Usage
	Chars   int // tool_args: argument characters generated so far
}

// Config configures one agent run.
type Config struct {
	// Roots is the config and state this agent and its children use; the
	// zero value is the user's own ugh installation.
	Roots      roots.Roots
	AllowSpawn bool     // parent explicitly permits delegation by this child
	Outputs    []string // expected paths, relative to Cwd

	Provider llm.Provider
	Model    string
	System   string
	Thinking string
	Tools    *tools.Registry
	Cwd      string
	MaxTurns int
	Out      io.Writer // assistant text stream (nil discards)
	Trace    io.Writer // tool-call trace lines (nil discards)
	Gate     perm.Gate // permission gate (nil allows everything, M1 style)
	// Agent is the tree path reported to the permission system (default
	// "root"; subagents set their own path in M6).
	Agent string
	// History seeds the conversation (sessions resume); Run continues it.
	History []llm.Message
	// Events receives progress notifications from the run goroutine.
	Events func(Event)
	Tree   *Tree
	Node   *Node
	Spawn  func(context.Context, *Agent, []Task) string
	// StartChild runs one child to completion; with it set, spawn runs
	// children in the background (async) unless the call asks to wait.
	StartChild func(context.Context, *Agent, Task) ChildResult
	// ProviderName names Provider for agents without Routes (the root), so a
	// quota refusal still marks the provider exhausted.
	ProviderName string
	// Resume continues a finished child (resume_agent).
	Resume func(ctx context.Context, parent *Agent, id, task string, maxTurns int) ChildResult
	Routes []string
	// StepDown is the index in Routes where the next tier down begins (a
	// high-tier child's low-tier tail); 0 when Routes is all one tier.
	StepDown    int
	StreamLimit func(string) chan struct{}
	// OnMessage is called with every message appended to the history (user
	// prompt, assistant turn, tool result) so sessions can record it.
	OnMessage func(llm.Message)
	// AutoCompact (0 disables) compacts older turns once the last prompt
	// passed this fraction of the context window (ContextWindow, or the
	// model card's).
	AutoCompact   float64
	ContextWindow int
	// CompactThreshold, when positive, is this agent's own absolute compaction
	// size (prompt tokens); it wins over every configured value (see limitFor).
	CompactThreshold int
	// BeforeRequest is called before each request attempt (every route, every
	// retry) with its worst-case estimate. Returning an error stops the turn
	// with ErrBudget. release is called exactly once per successful reserve,
	// on every exit path, with the usage the attempt actually reported.
	BeforeRequest func(est llm.Estimate) (release func(actual llm.Usage), err error)
	// MaxOutputTokens is the output size estimates assume (0: 16384).
	MaxOutputTokens int
	// OnCompact receives the new, compacted history so sessions can record it.
	OnCompact func([]llm.Message)
	// OnStatus records a status change worth showing outside the process
	// ("waiting on <children>", "" when it ends) in the session file.
	OnStatus func(string)
	// ToolTimeout bounds file tools (0: 120 s); see runTool.
	ToolTimeout time.Duration
	// Scratch is this agent's private scratch directory ($UGH_SCRATCH).
	Scratch string
	// Inbox is this agent's `ugh send` queue (session.InboxPath); checked
	// before every turn and while waiting on children.
	Inbox string
	// Deadline, when set, is a wall-clock budget: once passed, the next turn
	// becomes the graceful turn-out (nudge, no tools, report).
	Deadline time.Time
	// Admission is the harness-set premium quota context (see boundary.go).
	Admission Admission
	// Runtime lets the spawn tool follow routing mode (managed vs traditional).
	Runtime *Runtime
	// OnAdmitted fires each time a request attempt has its provider and premium
	// slots (after any queueing), so callers can start execution timers there.
	OnAdmitted func()
	// OnRecord receives structured session records (e.g. council outcomes).
	OnRecord func(session.Record)
	// Archive is the resumed session's chronological records (Loaded.Display);
	// it seeds the tool-result archive that recall reads.
	Archive []session.Record
	// Context shapes what each turn shows the model (see ContextConfig).
	Context ContextConfig
	// OneShot makes a tool-less, single-turn helper agent: no tools, ladder,
	// instructions, digest or compaction, whatever else is configured.
	OneShot bool
	// BlobDir keeps the full text of trimmed tool output (<session>.blobs);
	// "" keeps none.
	BlobDir string
}

// ErrTurnOut marks a run that exhausted its turn or time budget. Run still
// returns a report alongside it (see TurnOutError), never an empty result.
var ErrTurnOut = errors.New("turn budget exhausted")

// TurnOutError carries the turn-out details; errors.Is(err, ErrTurnOut).
type TurnOutError struct {
	Turns    int
	Answered bool // the model gave a final text answer after the nudge
}

func (e *TurnOutError) Error() string {
	return fmt.Sprintf("turn budget exhausted after %d turns", e.Turns)
}
func (e *TurnOutError) Is(target error) bool { return target == ErrTurnOut }

// turnOutNudge is injected two turns before the budget runs out.
const turnOutNudge = "Turn budget nearly exhausted: write your state to disk, commit if this is a " +
	"git repository, and reply with your final report now."

// Agent runs the loop and accumulates token usage across turns.
type Agent struct {
	*Node       // live, thread-safe tree metadata (snapshot through Tree.Snapshot)
	cfg         Config
	hist        []llm.Message
	task        string // current prompt, reported as classifier context
	overflow    bool   // the next premium admission may use the overflow limit
	Usage       llm.Usage
	ToolCount   int
	Turns       int            // model turns taken across all Run calls
	toolUse     map[string]int // tool name → calls, for the turn-out summary
	Compactions int
	lastIn      int // prompt tokens of the latest turn
	lastInPub   atomic.Int64
	// longWaits counts model calls separated from their predecessor by more
	// than llm.CacheLongGap (subagent waits and other long tools); once the
	// conversation shows such pauses the prompt cache needs the 1h TTL.
	longWaits    int
	lastCallAt   time.Time
	tokPerByte   float64 // calibrated prompt tokens per request byte
	lastReqBytes int     // size of the request last sent
	compactHold  int     // do not compact again before the history reaches this length
	limitMu      sync.Mutex
	limitCache   map[string]limitResult
	compactFails int
	critical     criticalState // verbatim task/path state, never delegated to the summarizer
	stopReason   string
	liveSlot     bool // holds a Runtime live-agent slot
	// routeCooldown holds routes left after a transient failure; they are
	// skipped until the time passes, then preferred again.
	routeCooldown map[string]time.Time
	kidsMu        sync.Mutex
	kidsCtx       context.Context // children run under it; cancelled when the time budget ends
	kidsCancel    context.CancelFunc
	kids          []*childRun   // background children not yet delivered
	wake          chan struct{} // child finished or user message queued
	rt            *Runtime      // set for spawned children (live-slot release)
	inboxOff      int64
	maxMinutes    int // per-task wall-clock budget from spawn
	councilMu     sync.Mutex
	councils      map[string]*CouncilRecordMeta // council outcomes by id (this agent convened them)
	routeNote     string                        // routing decision to report with the result
	thinkingWant  string                        // thinking level asked for; re-clamped when the route changes
	arch          archive                       // every tool result of this conversation, by ordinal
	instr         map[string]seenInstr          // AGENTS.md files already shown, by real path
	view          map[int]viewLevel             // ladder level by tool-result ordinal; absent = full
	viewDirty     bool                          // a new query arrived; decide the view before the next request
	query         string                        // what the user currently wants (prompt, steer or inbox message)
	digestMu      sync.Mutex
	digests       map[int]string // digest text by ordinal, computed at most once; guarded by digestMu
	lastLadder    ladderStats
	ladderFails   int
	jevOnce       sync.Once
	jev           model.CostAsker
	steerMu       sync.Mutex
	steered       []string // user messages queued while running
	// Denied lists permission denials ("tool args: reason") so unattended
	// runs can surface them in their final summary.
	Denied    []string
	lastCalls []string // the most recent tool calls, one line each
}

// New returns an agent with defaults applied.
func New(cfg Config) *Agent {
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.Trace == nil {
		cfg.Trace = io.Discard
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 24
	}
	if cfg.OneShot {
		cfg.Tools, cfg.Context, cfg.AutoCompact = tools.NewRegistry(), ContextConfig{}, 0
		cfg.Spawn, cfg.StartChild, cfg.Resume = nil, nil, nil
		cfg.MaxTurns = 1
	}
	if cfg.Tools == nil { // tool-less agents (e.g. /compact summarizer)
		cfg.Tools = tools.NewRegistry()
	}
	if cfg.Agent == "" {
		cfg.Agent = "root"
	}
	if cfg.Tree == nil {
		cfg.Tree = &Tree{}
	}
	if cfg.Node == nil {
		// The root node carries the full ref like every child, and a rebuilt
		// root (/model) updates it: admission, stream limits and history
		// adaptation all read the node's model.
		ref := cfg.Model
		if cfg.ProviderName != "" {
			ref = cfg.ProviderName + "/" + cfg.Model
		}
		cfg.Node = cfg.Tree.Root(cfg.Agent, ref, cfg.Cwd)
		cfg.Node.setModel(ref)
	}
	a := &Agent{Node: cfg.Node, cfg: cfg, hist: reconcileHistory(cfg.History), wake: make(chan struct{}, 1)}
	a.arch.seed(cfg.Archive, cfg.History, cfg.BlobDir)
	if cfg.Context.Instructions && !cfg.OneShot {
		a.cfg.System += rootInstructions(cfg.Cwd)
		a.restoreInstructions()
	}
	cfg.Node.setAgent(a)
	a.rt = cfg.Runtime
	a.restoreCritical()
	a.thinkingWant = cfg.Thinking
	if cfg.Context.ladderOn() && !cfg.OneShot {
		cfg.Tools.Add(recallTool{agent: a})
	}
	if cfg.Context.AskUser != nil {
		cfg.Tools.Add(askUserTool{agent: a}) // any agent may ask; no UI (headless) means no tool
	}
	if cfg.Spawn != nil && a.canSpawn() {
		cfg.Tools.Add(spawnTool{agent: a})
		if cfg.StartChild != nil {
			cfg.Tools.Add(waitTool{agent: a})
			cfg.Tools.Add(statusTool{agent: a})
		}
		if cfg.Resume != nil {
			cfg.Tools.Add(resumeTool{agent: a})
		}
	}
	if cfg.Spawn != nil && (cfg.Node.Role == "root" || cfg.Node.Role == "orchestrator") {
		cfg.Tools.Add(worktreeStatusTool{agent: a})
		cfg.Tools.Add(worktreeIntegrateTool{agent: a})
		cfg.Tools.Add(worktreeCleanupTool{agent: a})
	}
	// The council tool is root-only (§6): the root calls it once on a design
	// question; the panel seats run through the spawn machinery underneath it.
	if cfg.Spawn != nil && cfg.Node.Depth == 0 && cfg.Node.Role == "root" {
		cfg.Tools.Add(councilTool{agent: a})
		cfg.Tools.Add(NewGutcheckTool(nil)) // advisory only; never an approval or routing authority
	}
	return a
}

// History returns the accumulated conversation (seed + all turns so far).
func (a *Agent) History() []llm.Message { return a.hist }

// Run executes one prompt to completion and returns the final assistant
// text. Repeated calls continue the same conversation history.
//
// Budget: two turns before MaxTurns (or once Deadline passes) a nudge asks
// the model to wrap up; the last turn offers no tools. A run that still ends
// without an answer returns a "[TURN-OUT]" report built from the last
// assistant text and a tool-use summary, with a *TurnOutError.
func (a *Agent) Run(ctx context.Context, prompt string) (string, error) {
	text, err := a.runLoop(ctx, prompt)
	// A run that ends early (turn-out, error, stop) still collects every
	// background child, so no work is orphaned or lost.
	if a.pendingKids() > 0 && !a.cfg.Deadline.IsZero() && time.Now().After(a.cfg.Deadline) {
		// Out of time: do not wait on children that may be stuck.
		fmt.Fprintf(a.cfg.Trace, "⏳ %s: time budget spent; cancelling %d subagent(s)\n", a.cfg.Agent, a.pendingKids())
		a.cancelKids()
	}
	if a.pendingKids() > 0 {
		fmt.Fprintf(a.cfg.Trace, "⏳ %s: waiting for %d subagent(s) before returning\n", a.cfg.Agent, a.pendingKids())
		if rest := a.drainKids(ctx); rest != "" {
			text += "\n\n[Subagent results that arrived after this agent stopped]\n\n" + rest
		}
	}
	return text, err
}

func (a *Agent) runLoop(ctx context.Context, prompt string) (string, error) {
	specs := a.cfg.Tools.Specs()
	a.cfg.Node.setStatus("running")
	status := "error"
	defer func() { a.cfg.Node.setStatus(status) }()
	a.task = prompt
	a.pinTask(prompt)
	a.query, a.viewDirty = prompt, true
	a.push(llm.Message{Role: llm.RoleUser, Text: prompt})
	// The nudge lands two turns before the end (on the final turn for tiny
	// budgets). Tool-less agents (summarizers) have no budget to manage.
	nudgeAt := max(a.cfg.MaxTurns-2, 1)
	if len(specs) == 0 {
		nudgeAt = a.cfg.MaxTurns + 1
	}
	nudged, lastText, start := false, "", a.Turns
	for turn := 1; turn <= a.cfg.MaxTurns; turn++ {
		specs = a.cfg.Tools.Specs() // tool schemas can change mid-run (spawn exposes model/thinking during a router outage)
		final := turn == a.cfg.MaxTurns
		late := !a.cfg.Deadline.IsZero() && time.Now().After(a.cfg.Deadline)
		if late {
			final = true
		}
		if !nudged && (turn >= nudgeAt || late) {
			nudged = true
			fmt.Fprintf(a.cfg.Trace, "⏳ %s: turn-out nudge at turn %d/%d\n", a.cfg.Agent, turn, a.cfg.MaxTurns)
			a.push(llm.Message{Role: llm.RoleUser, Text: turnOutNudge}.WithSource(llm.SourceHarness))
		}
		a.pollInbox()
		for _, text := range a.TakeSteered() {
			a.pinTask(text)
			a.push(llm.Message{Role: llm.RoleUser, Text: "[Message from the user, sent while you were working]\n" + text})
			a.query, a.viewDirty = text, true // a steer is a new query: decide the view again
		}
		a.deliverKids()
		if a.viewDirty {
			a.decideView(ctx)
		}
		a.maybeCompact(ctx, specs)
		a.Turns++
		assistant, calls, err := a.finalOrStream(ctx, final, specs)
		if err != nil {
			if ctx.Err() != nil {
				status = "cancelled"
			}
			return "", err
		}
		if final && len(calls) == 0 && len(specs) > 0 {
			// With no tools offered some models write their calls as text
			// markup; that is not a final answer.
			if before, ok := cutToolMarkup(assistant.Text); ok {
				assistant.Text = before
				a.push(assistant)
				if strings.TrimSpace(before) != "" {
					lastText = before
				}
				break
			}
		}
		if strings.TrimSpace(assistant.Text) != "" {
			lastText = assistant.Text
		}
		if final && len(calls) > 0 {
			// No tools were offered: never run what the model asked for, keep
			// the history free of unanswered calls, and report the turn-out
			// (text next to a tool call is progress, not a final answer).
			assistant.ToolCalls = nil
			a.push(assistant)
			break
		}
		a.push(assistant)
		if len(calls) == 0 {
			if strings.TrimSpace(assistant.Text) == "" && final {
				break
			}
			if a.pendingKids() > 0 && !final {
				// A final answer with children still out: wait for a result or a
				// user message, then continue (waiting costs no turn).
				fmt.Fprintf(a.cfg.Trace, "⏳ %s: waiting on %d subagent(s)\n", a.cfg.Agent, a.pendingKids())
				if len(a.collectPeek()) == 0 && !a.hasSteered() {
					a.block(ctx, 0)
				}
				if ctx.Err() != nil {
					status = "cancelled"
					return "", ctx.Err()
				}
				turn--
				continue
			}
			if nudged {
				status = "turn-out"
				return "[TURN-OUT] " + assistant.Text, &TurnOutError{Turns: a.Turns - start, Answered: true}
			}
			status = "done"
			a.compactAtBoundary(ctx, specs)
			return assistant.Text, nil
		}
		for _, call := range calls {
			result, stop, err := a.execute(ctx, call)
			if err != nil {
				if ctx.Err() != nil {
					status = "cancelled"
				}
				return "", err
			}
			if stop {
				return "", fmt.Errorf("%w: %s", perm.ErrStopped, a.stopReason)
			}
			a.push(llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, ToolName: call.Name, Text: result})
		}
	}
	status = "turn-out"
	return a.turnOutReport(lastText), &TurnOutError{Turns: a.Turns - start}
}

// finalOrStream streams one turn; the final turn offers no tools. Providers
// that reject a tool-less request after tool history get one retry with
// tools offered (any calls are then dropped by the caller).
func (a *Agent) finalOrStream(ctx context.Context, final bool, specs []llm.ToolSpec) (llm.Message, []llm.ToolCall, error) {
	if !final {
		return a.stream(ctx, a.hist, specs)
	}
	msg, calls, err := a.stream(ctx, a.hist, nil)
	if err != nil && ctx.Err() == nil && len(specs) > 0 && !errors.Is(err, ErrBudget) {
		return a.stream(ctx, a.hist, specs)
	}
	return msg, calls, err
}

// pollInbox moves messages queued by `ugh send` into the steer queue; it
// reports whether any arrived.
func (a *Agent) pollInbox() bool {
	if a.cfg.Inbox == "" {
		return false
	}
	msgs, off := session.ReadInbox(a.cfg.Inbox, a.inboxOff)
	a.inboxOff = off
	for _, m := range msgs {
		a.Steer(m.Text)
	}
	return len(msgs) > 0
}

// Steer queues a user message for this agent's next turn; safe to call from
// another goroutine while Run is in progress.
func (a *Agent) Steer(text string) {
	a.steerMu.Lock()
	a.steered = append(a.steered, text)
	a.steerMu.Unlock()
	a.signal()
}

func (a *Agent) hasSteered() bool {
	a.steerMu.Lock()
	defer a.steerMu.Unlock()
	return len(a.steered) > 0
}

// collectPeek lists finished-but-undelivered children without taking them.
func (a *Agent) collectPeek() []*childRun {
	var out []*childRun
	for _, c := range a.kidsSnapshot() {
		if c.finished() {
			out = append(out, c)
		}
	}
	return out
}

// TakeSteered returns and clears the queued messages.
func (a *Agent) TakeSteered() []string {
	a.steerMu.Lock()
	defer a.steerMu.Unlock()
	out := a.steered
	a.steered = nil
	return out
}

// lastText is the most recent non-empty assistant text in the history.
func (a *Agent) lastText() string {
	for i := len(a.hist) - 1; i >= 0; i-- {
		if a.hist[i].Role == llm.RoleAssistant && strings.TrimSpace(a.hist[i].Text) != "" {
			return a.hist[i].Text
		}
	}
	return ""
}

// toolMarkers are text forms of tool calls that models emit when no tools
// are offered (GLM/Qwen/Hermes-style tags, raw JSON tool calls).
var toolMarkers = []string{"<tool_call>", "<tool_calls>", "<function_call", "<|tool_call", `"tool_calls":`, "<invoke "}

// cutToolMarkup reports whether text contains a textual tool call and
// returns the prose before it.
func cutToolMarkup(text string) (string, bool) {
	cut := -1
	for _, m := range toolMarkers {
		if i := strings.Index(text, m); i >= 0 && (cut < 0 || i < cut) {
			cut = i
		}
	}
	if cut < 0 {
		return text, false
	}
	return strings.TrimSpace(text[:cut]), true
}

// push appends one message to the history and records it.
func (a *Agent) push(m llm.Message) {
	switch m.Role {
	case llm.RoleAssistant:
		a.arch.noteCalls(m)
	case llm.RoleTool:
		m.Ord = a.arch.add(m, a.cfg.BlobDir).Ord
	}
	if m.Meta[llm.MetaSource] == "" && (m.Role == llm.RoleTool || m.Role == llm.RoleUser) {
		m = m.WithSource(m.Source())
	}
	a.hist = append(a.hist, m)
	a.record(m)
}

// turnOutReport is the parent-facing result when the budget ran out without
// a final answer: the last assistant text plus what the agent did.
func (a *Agent) turnOutReport(lastText string) string {
	var b strings.Builder
	if !a.cfg.Deadline.IsZero() && time.Now().After(a.cfg.Deadline) {
		fmt.Fprintf(&b, "[TURN-OUT] %s ran out of its time budget without a final answer.\n", a.cfg.Agent)
	} else {
		fmt.Fprintf(&b, "[TURN-OUT] %s ran out of its %d-turn budget without a final answer.\n", a.cfg.Agent, a.cfg.MaxTurns)
	}
	if t := strings.TrimSpace(lastText); t != "" {
		b.WriteString("Last assistant text:\n" + t + "\n")
	} else {
		b.WriteString("Last assistant text: (none)\n")
	}
	b.WriteString(a.toolSummary())
	return b.String()
}

// toolSummary is a one-paragraph account of tool use for turn-out reports.
func (a *Agent) toolSummary() string {
	if a.ToolCount == 0 {
		return "Tool use: none.\n"
	}
	names := make([]string, 0, len(a.toolUse))
	for n := range a.toolUse {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return a.toolUse[names[i]] > a.toolUse[names[j]] })
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = fmt.Sprintf("%s %d", n, a.toolUse[n])
	}
	return fmt.Sprintf("Tool use: %d calls (%s). Last calls:\n- %s\n", a.ToolCount, strings.Join(parts, ", "), strings.Join(a.lastCalls, "\n- "))
}

// record hands one finalized message to the session recorder.
func (a *Agent) record(m llm.Message) {
	a.cfg.Node.addMessage(m)
	if a.cfg.OnMessage != nil {
		a.cfg.OnMessage(m)
	}
}

// emit sends one progress notification to the UI.
func (a *Agent) emit(ev Event) {
	if a.cfg.Events != nil {
		a.cfg.Events(ev)
	}
}

// resolveRoute turns a provider/model ref into a client (tests replace it).
var resolveRoute = provider.Resolve

// resolve builds a route's client for this agent's roots. An instance with its
// own roots resolves against them; the user's installation goes through
// resolveRoute.
func (a *Agent) resolve(ref string) (llm.Provider, string, error) {
	return resolveIn(a.cfg.Roots, ref)
}

func resolveIn(r roots.Roots, ref string) (llm.Provider, string, error) {
	if !r.Explicit() {
		return resolveRoute(ref)
	}
	return provider.ResolveIn(r, ref)
}

// retryBackoff is the wait before each retry of a turn that failed
// transiently (stalled or broken stream, 429, 5xx).
var retryBackoff = []time.Duration{5 * time.Second, 20 * time.Second}

// transientCooldown is how long a route that failed transiently (after its
// one retry) is skipped before the agent tries it again.
var transientCooldown = 2 * time.Minute

// stream consumes one provider turn, printing text as it arrives. Transient
// failures retry the whole turn (partial output is discarded, never
// recorded).
func (a *Agent) stream(ctx context.Context, msgs []llm.Message, specs []llm.ToolSpec) (llm.Message, []llm.ToolCall, error) {
	for attempt := 0; ; attempt++ {
		msg, calls, err := a.streamRoutes(ctx, a.hist, specs)
		if err == nil || ctx.Err() != nil || attempt >= len(retryBackoff) || !transient(err) {
			return msg, calls, err
		}
		fmt.Fprintf(a.cfg.Trace, "↻ %s: %v — retrying in %s\n", a.cfg.Agent, err, retryBackoff[attempt])
		select {
		case <-time.After(retryBackoff[attempt]):
		case <-ctx.Done():
			return msg, calls, err
		}
	}
}

// queuedAnnounceAfter is how long a wait for a provider slot or admission
// must last before it is written to the session file as "queued".
const queuedAnnounceAfter = 250 * time.Millisecond

// transient reports whether a failed turn is worth retrying as-is.
func transient(err error) bool { return ClassifyFailure(err).Retryable() }

func (a *Agent) streamRoutes(ctx context.Context, msgs []llm.Message, specs []llm.ToolSpec) (llm.Message, []llm.ToolCall, error) {
	var last error
	reason := ""
	keepEarlier := false // an earlier route failed only transiently: do not drop it
	for i, route := range a.cfg.Routes {
		if i > 0 && reason != "" && a.cfg.OnRecord != nil {
			a.cfg.OnRecord(session.Record{Type: "route_fallback", Agent: a.cfg.Agent, Meta: map[string]any{
				"from": routeRef(a.cfg.Routes[i-1]), "to": routeRef(route), "reason": reason,
			}})
		}
		reason = ""
		prov, _, _ := strings.Cut(route, ":")
		if st, bad := provstate.For(a.cfg.Roots).Get(prov); bad && st.Kind == "disabled" && i < len(a.cfg.Routes)-1 {
			reason = "ineligible: provider disabled"
			last = fmt.Errorf("%s is turned off (/provider %s on)", prov, prov)
			continue
		}
		if st, bad := provstate.For(a.cfg.Roots).Get(prov); bad && st.Kind == "exhausted" && i < len(a.cfg.Routes)-1 {
			reason = "quota: provider exhausted"
			last = fmt.Errorf("%s exhausted until %s (%s)", prov, st.Until.Local().Format("15:04"), st.Reason)
			continue
		}
		if until, ok := a.routeCooldown[route]; ok && i < len(a.cfg.Routes)-1 {
			if time.Now().Before(until) {
				keepEarlier = true
				reason = "transient: route cooldown"
				continue
			}
			delete(a.routeCooldown, route)
		}
		if ref := routeRef(route); ref != a.cfg.Node.Model {
			p, m, e := a.resolve(ref)
			if e != nil {
				last = e
				reason = "ineligible: route unavailable"
				continue
			}
			// A route switch re-resolves the limit for the new model: a
			// smaller-window failover compacts instead of being skipped.
			if limit, lok := a.limitFor(ref); lok {
				if est := a.estimateTokens(msgs, specs); est >= limit && a.compactFor(ctx, ref, limit, est, specs) {
					msgs = a.hist
				}
			}
			if w, ok := a.windowFor(ref); ok {
				if est := a.estimateTokens(msgs, specs); est > w {
					reason = "ineligible: context window"
					last = fmt.Errorf("context of ~%d tokens does not fit %s (window %d)", est, ref, w)
					continue
				}
			}
			fmt.Fprintf(a.cfg.Trace, "⇄ %s: switching to %s\n", a.cfg.Agent, ref)
			a.cfg.Provider, a.cfg.Model = p, m
			if levels, known := ModelThinking(ref); known && a.thinkingWant != "" {
				want := a.thinkingWant
				if c := thinkingCap(a.routingConfig(), ref); indexOf(ThinkingLevels, want) > c {
					want = ThinkingLevels[c] // e.g. Sol at xhigh failing over to Haiku (≤ high)
				}
				a.cfg.Thinking = clampLevel(levels, want) // requested intent, re-mapped per route
				fmt.Fprintf(a.cfg.Trace, "  thinking: requested %s, applied %s\n", a.thinkingWant, a.cfg.Thinking)
			}
			a.cfg.Node.setModel(ref)
		}
		msg, calls, started, err := a.streamOnce(ctx, msgs, specs)
		if err != nil && !started && ClassifyFailure(err) == FailAuth && refreshAuth(ctx, prov) {
			fmt.Fprintf(a.cfg.Trace, "🔑 %s: %s credentials refreshed after rejection; retrying\n", a.cfg.Agent, prov)
			msg, calls, started, err = a.streamOnce(ctx, msgs, specs)
		}
		if err != nil && !started && ClassifyFailure(err) == FailCapacity && a.lastOfTier(i) {
			// The pool is full and this tier has nothing else: the limit would
			// now cause the downtime it exists to avoid, so borrow the next
			// preset's limit once before stepping down a tier.
			fmt.Fprintf(a.cfg.Trace, "⇡ %s: premium pool full and %s is the tier's last route; admitting over the limit\n", a.cfg.Agent, route)
			a.overflow = true
			msg, calls, started, err = a.streamOnce(ctx, msgs, specs)
			a.overflow = false
		}
		if err != nil && !started && ClassifyFailure(err) == FailTransient && i < len(a.cfg.Routes)-1 && len(retryBackoff) > 0 {
			// One hiccup should not move the agent to a (possibly paid) fallback.
			fmt.Fprintf(a.cfg.Trace, "↻ %s: %v — retrying %s once before failing over\n", a.cfg.Agent, err, route)
			select {
			case <-time.After(retryBackoff[0]):
				msg, calls, started, err = a.streamOnce(ctx, msgs, specs)
			case <-ctx.Done():
				return msg, calls, err
			}
		}
		if err == nil {
			if !keepEarlier {
				a.cfg.Routes = a.cfg.Routes[i:]
				a.cfg.StepDown = max(a.cfg.StepDown-i, 0)
			}
			noteRouteSuccess(a.cfg.Node.Model)
			return msg, calls, nil
		}
		last = err
		reason = ClassifyFailure(err).String()
		if ClassifyFailure(err) == FailCapacity {
			reason = "admission pool full"
		}
		if !started && ClassifyFailure(err) == FailTransient && i < len(a.cfg.Routes)-1 {
			if a.routeCooldown == nil {
				a.routeCooldown = map[string]time.Time{}
			}
			a.routeCooldown[route] = time.Now().Add(transientCooldown)
			keepEarlier = true
		}
		if until, reason, ok := provstate.QuotaError(err); ok {
			provstate.For(a.cfg.Roots).Mark(prov, provstate.State{Kind: "exhausted", Until: until, Reason: reason})
			fmt.Fprintf(a.cfg.Trace, "⛁ %s: %s exhausted until %s (%s)\n", a.cfg.Agent, prov, until.Local().Format("15:04"), reason)
		}
		if started || !providerFailure(err) || i == len(a.cfg.Routes)-1 {
			return msg, calls, err
		}
	}
	if last != nil {
		return llm.Message{}, nil, last
	}
	m, c, started, e := a.streamOnce(ctx, msgs, specs)
	if e != nil && !started && ClassifyFailure(e) == FailAuth && refreshAuth(ctx, a.cfg.ProviderName) {
		m, c, _, e = a.streamOnce(ctx, msgs, specs)
	}
	if e == nil && a.cfg.ProviderName != "" {
		noteRouteSuccess(a.cfg.ProviderName + "/" + a.cfg.Model) // a root's completed request is access evidence too
	}
	if until, reason, ok := provstate.QuotaError(e); ok && a.cfg.ProviderName != "" {
		provstate.For(a.cfg.Roots).Mark(a.cfg.ProviderName, provstate.State{Kind: "exhausted", Until: until, Reason: reason})
	}
	return m, c, e
}

// lastOfTier reports whether Routes[i] is the last route of the agent's own
// tier (the step-down tail, if any, belongs to the tier below).
func (a *Agent) lastOfTier(i int) bool {
	end := len(a.cfg.Routes)
	if a.cfg.StepDown > 0 && a.cfg.StepDown < end {
		end = a.cfg.StepDown
	}
	return i == end-1
}

func providerFailure(err error) bool { return ClassifyFailure(err).FailsOver() }
func (a *Agent) streamOnce(ctx context.Context, msgs []llm.Message, specs []llm.ToolSpec) (llm.Message, []llm.ToolCall, bool, error) {
	started := false
	// While waiting for a provider slot or premium admission the agent is
	// "queued" (tree pane, `ugh watch`); defers restore the status on admission,
	// cancellation and failover alike.
	prevStatus := a.cfg.Node.status()
	var qmu sync.Mutex
	queuedNow, announced := false, false
	var qtimer *time.Timer
	markQueued := func() {
		qmu.Lock()
		defer qmu.Unlock()
		if queuedNow {
			return
		}
		queuedNow = true
		a.cfg.Node.setStatus("queued")
		if a.cfg.OnStatus != nil {
			// The session file hears about it only if the wait is real.
			qtimer = time.AfterFunc(queuedAnnounceAfter, func() {
				qmu.Lock()
				defer qmu.Unlock()
				if queuedNow && !announced {
					announced = true
					a.cfg.OnStatus("queued")
				}
			})
		}
	}
	unqueue := func() {
		qmu.Lock()
		defer qmu.Unlock()
		if !queuedNow {
			return
		}
		queuedNow = false
		if qtimer != nil {
			qtimer.Stop()
		}
		a.cfg.Node.setStatus(prevStatus)
		if announced {
			announced = false
			a.cfg.OnStatus("")
		}
	}
	defer unqueue()
	// Acquisition order: provider stream limit, then premium admission. Defers
	// run in reverse: cancel+drain the attempt, release admission, release the
	// provider slot.
	if a.cfg.StreamLimit != nil {
		name, _, _ := strings.Cut(a.cfg.Node.Model, "/")
		sem := a.cfg.StreamLimit(name)
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			markQueued()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return llm.Message{}, nil, false, ctx.Err()
			}
		}
	}
	markQueued()
	release, err := a.admit(ctx)
	if err != nil {
		return llm.Message{}, nil, false, err
	}
	defer release()
	unqueue()
	if a.cfg.OnAdmitted != nil {
		a.cfg.OnAdmitted()
	}
	sent := a.assemble(msgs)
	a.lastReqBytes = requestBytes(a.cfg.System, sent, specs)
	releaseBudget, err := a.reserveBudget(a.lastReqBytes)
	if err != nil {
		return llm.Message{}, nil, false, err
	}
	var spent llm.Usage
	defer func() { releaseBudget(spent) }()
	actx, cancel := context.WithCancel(ctx) // each attempt has its own lifetime
	a.cfg.Node.clearPartial()               // a retried turn starts its live view over
	if !a.lastCallAt.IsZero() && time.Since(a.lastCallAt) > llm.CacheLongGap {
		a.longWaits++
	}
	a.lastCallAt = time.Now()
	ch, err := a.cfg.Provider.Stream(actx, llm.Request{
		Model: a.cfg.Model, System: a.cfg.System, Messages: sent,
		Tools: specs, Thinking: a.cfg.Thinking, Foreground: a.cfg.Agent == "root",
		CacheTTL: llm.CacheTTL(a.longWaits),
	})
	if err != nil {
		cancel()
		return llm.Message{}, nil, false, err
	}
	defer func() { cancel(); drain(ch) }()
	acc := chat.NewCallAccumulator()
	writingTool, argChars := "", 0
	var text, reasoning strings.Builder
	var native []string
	for ev := range ch {
		switch ev.Type {
		case llm.EventTextDelta:
			started = true
			text.WriteString(ev.TextDelta)
			a.cfg.Node.appendPartial(ev.TextDelta, "")
			fmt.Fprint(a.cfg.Out, ev.TextDelta)
			a.emit(Event{Kind: "text", Text: ev.TextDelta})
		case llm.EventThinkingDelta:
			started = true
			reasoning.WriteString(ev.Thinking)
			a.cfg.Node.appendPartial("", ev.Thinking)
			a.emit(Event{Kind: "thinking", Text: ev.Thinking})
		case llm.EventNative:
			started = true
			if ev.Native != "" {
				native = append(native, ev.Native)
			}
		case llm.EventToolCall:
			started = true
			if ev.Call != nil {
				acc.Add(ev.Seq, *ev.Call)
				// Large calls (a spawn with long briefs) take minutes to
				// generate; report progress so the agent does not look stuck.
				if ev.Call.Name != "" {
					writingTool = ev.Call.Name
				}
				argChars += len(ev.Call.Args)
				a.cfg.Node.setWriting(writingTool, argChars)
				a.emit(Event{Kind: "tool_args", Call: llm.ToolCall{Name: writingTool}, Chars: argChars})
			}
		case llm.EventUsage:
			if ev.Usage != nil {
				a.lastIn = ev.Usage.In
				a.lastInPub.Store(int64(ev.Usage.In))
				if ev.Usage.In > 0 && a.lastReqBytes > 0 {
					a.tokPerByte = float64(ev.Usage.In) / float64(a.lastReqBytes)
				}
				spent.Add(*ev.Usage)
				a.Usage.In += ev.Usage.In
				a.Usage.Out += ev.Usage.Out
				a.Usage.Cached += ev.Usage.Cached
				a.Usage.Cost += ev.Usage.Cost
				a.cfg.Node.setUsage(a.Usage)
				a.emit(Event{Kind: "usage", Usage: a.Usage})
				role := a.cfg.Node.Role
				if a.cfg.OneShot {
					role = "helper" // digests, compaction: delegated, whatever node they hang off
				}
				usage.NewLedger(a.cfg.Roots).RecordRoute(a.cfg.Node.Model, role, ev.Usage.In, ev.Usage.Out, ev.Usage.Cached, ev.Usage.Cost)
			}
		case llm.EventError:
			return llm.Message{}, nil, started, ev.Err
		}
	}
	if err := ctx.Err(); err != nil {
		return llm.Message{}, nil, started, err
	}
	if text.Len() > 0 {
		fmt.Fprintln(a.cfg.Out)
	}
	calls := acc.Finish()
	renamed := uniqueCallIDs(calls, a.Turns)
	msg := llm.Message{
		Role: llm.RoleAssistant, Text: text.String(),
		Reasoning: reasoning.String(), ToolCalls: calls, Route: a.cfg.Node.Model,
	}
	// Native items carry the provider's own call ids; once we renamed one they
	// would no longer pair with our results, so replay rebuilds from ToolCalls.
	if len(native) > 0 && !renamed {
		msg.Native = "[" + strings.Join(native, ",") + "]"
	}
	return msg, calls, started, nil
}

// execute judges one tool call with the permission gate and runs it when
// allowed. stop=true halts the entire run (the tool never runs).
func (a *Agent) execute(ctx context.Context, tc llm.ToolCall) (result string, stop bool, err error) {
	if note := a.mutationPathWarning(tc); note != "" {
		defer func() { result += "\n\n" + note }()
	}
	a.ToolCount++
	if a.toolUse == nil {
		a.toolUse = map[string]int{}
	}
	a.toolUse[tc.Name]++
	a.lastCalls = append(a.lastCalls, tc.Name+" "+summarize(tc.Args, 100))
	if len(a.lastCalls) > 5 {
		a.lastCalls = a.lastCalls[1:]
	}
	a.cfg.Node.setStatus("tool " + tc.Name)
	defer a.cfg.Node.setStatus("running")
	a.emit(Event{Kind: "tool_start", Call: tc})
	tool, ok := a.cfg.Tools.Get(tc.Name)
	if !ok {
		return fmt.Sprintf("error: unknown tool %q", tc.Name), false, nil
	}
	raw := json.RawMessage(tc.Args)
	if strings.TrimSpace(tc.Args) == "" {
		raw = json.RawMessage("{}")
	}
	// Instructions that apply to the path come first: a mutation they have not
	// been shown for does not run (its result carries them instead).
	blocked, readNote, readFiles := a.instructionGate(tc)
	if blocked != "" {
		fmt.Fprintf(a.cfg.Trace, "📘 %s %s deferred: AGENTS.md instructions not yet shown\n", tc.Name, summarize(tc.Args, 100))
		a.emit(Event{Kind: "tool_done", Call: tc, Output: "not executed — instructions shown first"})
		return blocked, false, nil
	}
	call := perm.NewCall(a.cfg.Cwd, tc.Name, raw)
	call.ID = tc.ID
	call.Agent = a.cfg.Agent
	call.Task = a.task
	call.Recent = a.recent()
	if v := a.ownerViolations(call); len(v) > 0 {
		reason := "feature writers may only run git inside their own worktree: " + strings.Join(v, "; ")
		a.Denied = append(a.Denied, fmt.Sprintf("%s %s: %s", tc.Name, summarize(tc.Args, 120), reason))
		a.emit(Event{Kind: "tool_done", Call: tc, Output: "⚠ denied", Elapsed: 0})
		return "permission denied: " + reason, false, nil
	}
	if a.cfg.Gate != nil {
		switch oc := a.cfg.Gate.Check(ctx, call); oc.Action {
		case perm.Stop:
			a.stopReason = fmt.Sprintf("%s %s (%s)", tc.Name, summarize(tc.Args, 120), oc.Reason)
			fmt.Fprintf(a.cfg.Trace, "⛔ %s stopped by permission policy (%s)\n", tc.Name, oc.Reason)
			a.emit(Event{Kind: "tool_done", Call: tc, Output: "⛔ " + oc.Reason, Elapsed: 0})
			return "", true, nil
		case perm.Defer:
			a.emit(Event{Kind: "tool_done", Call: tc, Output: "queued for approval #" + oc.Ticket})
			return "queued for approval #" + oc.Ticket + "; you will get the result as a message", false, nil
		case perm.Deny:
			a.Denied = append(a.Denied, fmt.Sprintf("%s %s: %s", tc.Name, summarize(tc.Args, 120), orReason(oc.Reason)))
			fmt.Fprintf(a.cfg.Trace, "⚠ %s denied (%s)\n", tc.Name, orReason(oc.Reason))
			a.emit(Event{Kind: "tool_done", Call: tc, Output: "⚠ denied", Elapsed: 0})
			return "permission denied: " + orReason(oc.Reason), false, nil
		}
	}
	fmt.Fprintf(a.cfg.Trace, "→ %s %s\n", tc.Name, summarize(tc.Args, 160))
	start := time.Now()
	if tc.Name == "bash" {
		ctx = tools.WithProgress(ctx, func(tail string) { // the live tail of a running command
			a.emit(Event{Kind: "tool_progress", Call: tc, Output: tail})
		})
	}
	out, runErr := a.runTool(ctx, tool, tc.Name, raw)
	elapsed := time.Since(start).Round(10 * time.Millisecond)
	// Screen the result for agent-directed instructions (§4.3). Errors are our
	// own text, so only real output is screened.
	if og, ok := a.cfg.Gate.(perm.OutputGate); ok && runErr == nil {
		og.CheckOutput(ctx, a.cfg.Agent, tc.Name, out)
	}
	if runErr != nil {
		fmt.Fprintf(a.cfg.Trace, "  error: %s (%s)\n", summarize(runErr.Error(), 160), elapsed)
		a.emit(Event{Kind: "tool_done", Call: tc, Output: "error: " + runErr.Error(), Err: runErr, Elapsed: elapsed})
		return "error: " + runErr.Error(), false, nil
	}
	a.noteCriticalCall(tc)
	out = a.maybeDigestFresh(ctx, tc, out)
	if readNote != "" {
		out += "\n\n" + readNote
		a.markSeen(readFiles)
	}
	fmt.Fprintf(a.cfg.Trace, "  ok (%s)\n", elapsed)
	a.emit(Event{Kind: "tool_done", Call: tc, Output: out, Elapsed: elapsed})
	return out, false, nil
}

// longTools run for as long as their work takes (children, councils).
var longTools = map[string]bool{"spawn": true, "wait_agents": true, "resume_agent": true, "council": true, "agent_status": true, "ask_user": true}

// runTool runs one tool under a deadline so a stuck call (a search over a
// huge tree, a FIFO, a hung filesystem) can never stall the agent: file
// tools get ToolTimeout, bash its own timeout plus a margin. On timeout the
// model is told to narrow the call.
func (a *Agent) runTool(ctx context.Context, tool tools.Tool, name string, raw json.RawMessage) (string, error) {
	ctx = tools.WithScratch(ctx, a.cfg.Scratch)
	_, hasRecall := a.cfg.Tools.Get("recall")
	ctx = tools.WithBlob(ctx, tools.Blob{Dir: a.cfg.BlobDir, Alias: aliasOf(a.arch.next()), Recall: hasRecall})
	if a.rt != nil {
		if wt := a.rt.worktreeAt(a.cfg.Cwd); wt != nil {
			ctx = tools.WithGroupTracker(ctx, func(pgid int) {
				tctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
				defer cancel()
				if err := wt.TrackGroup(tctx, pgid); err != nil {
					fmt.Fprintf(a.cfg.Trace, "  worktree: tracking background group %d failed: %v\n", pgid, err)
				}
			})
		}
	}
	if longTools[name] {
		return tools.Run(ctx, tool, a.cfg.Cwd, raw)
	}
	limit := a.cfg.ToolTimeout
	if limit <= 0 {
		limit = 120 * time.Second
	}
	if name == "bash" {
		limit = 615 * time.Second // bash enforces timeout_seconds (≤600) itself
	}
	tctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	type result struct {
		out string
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := tools.Run(tctx, tool, a.cfg.Cwd, raw)
		done <- result{out, err}
	}()
	select {
	case r := <-done:
		return r.out, r.err
	case <-tctx.Done():
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("%s timed out after %s; narrow the path or pattern, or split the work", name, limit)
	}
}

// recent returns the last few conversation turns as classifier context
// (§4.3 state.recent_messages): tool output is withheld, text is truncated.
func (a *Agent) recent() []perm.Message {
	const maxRecent, maxText = 6, 2000
	tail := a.hist
	if len(tail) > maxRecent {
		tail = tail[len(tail)-maxRecent:]
	}
	out := make([]perm.Message, 0, len(tail))
	for _, m := range tail {
		if m.Role == llm.RoleTool {
			out = append(out, perm.Message{Role: "toolResult", Text: "[prior tool output withheld]"})
			continue
		}
		text := m.Text
		if len(text) > maxText {
			text = text[:maxText]
		}
		out = append(out, perm.Message{Role: string(m.Role), Text: text})
	}
	return out
}

func orReason(s string) string {
	if s == "" {
		return "denied by permission policy"
	}
	return s
}

// summarize collapses whitespace and truncates s for one-line display.
func summarize(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}

// routingConfig is the runtime's config (the file's for agents without one).
func (a *Agent) routingConfig() config.Config {
	if a.rt != nil {
		return a.rt.Config
	}
	cfg, _ := config.LoadIn(a.cfg.Roots)
	return cfg
}
