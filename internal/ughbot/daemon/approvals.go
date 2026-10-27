package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/ughbot/bot"
	"github.com/AmadeusDE/Misa/internal/ughbot/config"
	"github.com/AmadeusDE/Misa/internal/ughbot/store"
)

// approvalTTL is how long a request waits; after it the request is denied.
const approvalTTL = 72 * time.Hour

const expireEvery = time.Minute

var _ api.Approvals = (*Daemon)(nil)
var _ bot.Proposer = (*Daemon)(nil)

func actionHash(action, args string) string {
	sum := sha256.Sum256([]byte(action + "\n" + args))
	return hex.EncodeToString(sum[:])
}

// ProposeBot checks a bot's request to create, change or delete a bot against the current
// bots and queues it for approval. Nothing changes until a person approves it.
func (d *Daemon) ProposeBot(from, action string, def config.Bot) (int64, error) {
	v := d.view.Load()
	old, exists := v.cfg.Bots[def.Name]
	var summary string
	switch action {
	case bot.ActionCreate:
		if exists {
			return 0, fmt.Errorf("%w: %q", api.ErrBotExists, def.Name)
		}
		if _, err := v.cfg.With(def, false); err != nil {
			return 0, err
		}
		summary = fmt.Sprintf("create bot %s: %s", def.Name, describe(def))
	case bot.ActionUpdate:
		if err := editable(v, def.Name, exists); err != nil {
			return 0, err
		}
		if _, err := v.cfg.With(def, false); err != nil {
			return 0, err
		}
		changes := changed(old, def)
		if changes == "" {
			return 0, errors.New("that changes nothing")
		}
		summary = fmt.Sprintf("change bot %s: %s", def.Name, changes)
	case bot.ActionDelete:
		if err := editable(v, def.Name, exists); err != nil {
			return 0, err
		}
		def = config.Bot{Name: def.Name}
		summary = "delete bot " + def.Name
	default:
		return 0, fmt.Errorf("unknown action %q", action)
	}
	return d.queue(from, action, def, summary)
}

// queue stores a request for approval: the exact action and arguments, bound by their hash.
func (d *Daemon) queue(from, action string, args any, summary string) (int64, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return 0, err
	}
	return d.st.AddApproval(store.Approval{Bot: from, Action: action, Args: string(raw),
		Hash: actionHash(action, string(raw)), Summary: summary, Expires: time.Now().Add(approvalTTL)})
}

// ProposeRoutine queues a routine from a bot without the schedule capability; the at time is
// fixed now, so what is approved is an absolute time.
func (d *Daemon) ProposeRoutine(from string, req api.RoutineRequest) (int64, error) {
	req.Owner = from
	r, err := d.resolve(req, time.Now())
	if err != nil {
		return 0, err
	}
	if !r.At.IsZero() {
		req.At = r.At.Format(time.RFC3339)
	}
	return d.queue(from, bot.ActionAddRoutine, req, "add routine: "+describeRoutine(r))
}

// ProposeCancel queues the deletion of a routine.
func (d *Daemon) ProposeCancel(from string, id int64) (int64, error) {
	r, err := d.st.Routine(id)
	if err != nil {
		return 0, noRoutine(err, id)
	}
	return d.queue(from, bot.ActionCancelRoutine, cancelArgs{ID: id}, fmt.Sprintf("delete routine #%d: %s", id, describeRoutine(r)))
}

// ProposeToolCall queues a tool call the permission classifiers flagged; approving runs it as is.
func (d *Daemon) ProposeToolCall(from, tool string, args json.RawMessage, why string) (int64, error) {
	summary := fmt.Sprintf("run %s %s", tool, compactJSON(args))
	if why != "" {
		summary += " (flagged: " + why + ")"
	}
	return d.queue(from, bot.ActionToolCall, toolCallArgs{Tool: tool, Args: args}, summary)
}

type cancelArgs struct {
	ID int64 `json:"id"`
}

type toolCallArgs struct {
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

func compactJSON(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return string(raw)
	}
	return b.String()
}

func describeRoutine(r store.Routine) string {
	when := "cron " + r.Cron
	if r.Cron == "" {
		when = "at " + r.At.Local().Format("2006-01-02 15:04")
	}
	if r.TZ != "" {
		when += " (" + r.TZ + ")"
	}
	prompt := r.Prompt
	if len([]rune(prompt)) > 80 {
		prompt = string([]rune(prompt)[:79]) + "…"
	}
	return fmt.Sprintf("mail %s %s, missed=%s: %q", r.Bot, when, r.Missed, prompt)
}

func editable(v *view, name string, exists bool) error {
	switch {
	case !exists:
		return fmt.Errorf("%w: %q", api.ErrUnknownBot, name)
	case !v.ui[name]:
		return api.ErrReadOnly
	}
	return nil
}

func (d *Daemon) Approvals(_ context.Context, state string) ([]api.ApprovalInfo, error) {
	d.expire()
	rows, err := d.st.Approvals(state, 200)
	if err != nil {
		return nil, err
	}
	out := make([]api.ApprovalInfo, len(rows))
	for i, a := range rows {
		out[i] = approvalInfo(a)
	}
	return out, nil
}

func approvalInfo(a store.Approval) api.ApprovalInfo {
	return api.ApprovalInfo{ID: a.ID, Bot: a.Bot, Action: a.Action, Summary: a.Summary, Args: json.RawMessage(a.Args),
		State: a.State, Created: a.Created, Expires: a.Expires, Decided: a.Decided, DecidedBy: a.DecidedBy, Result: a.Result}
}

// Approve carries out exactly the stored action, after checking it still matches its hash,
// and mails the outcome to the bot that asked.
func (d *Daemon) Approve(ctx context.Context, id int64, by string) (api.ApprovalInfo, error) {
	a, err := d.decide(id, store.ApprovalApproved, by)
	if err != nil {
		return api.ApprovalInfo{}, err
	}
	result, err := d.carryOut(ctx, a)
	if err != nil {
		result = err.Error()
	}
	a.Result = result
	if err != nil {
		a.State = store.ApprovalFailed
	}
	if serr := d.st.SetApprovalResult(id, result, err != nil); serr != nil {
		return approvalInfo(a), serr
	}
	verb := "approved"
	if err != nil {
		verb = "approved but failed"
	}
	d.tell(a, fmt.Sprintf("%s by %s: %s\n%s", verb, by, a.Summary, result))
	if err == nil && a.Action == bot.ActionAddRepo {
		d.startClone(a)
	}
	return approvalInfo(a), nil
}

func (d *Daemon) Deny(_ context.Context, id int64, by string) (api.ApprovalInfo, error) {
	a, err := d.decide(id, store.ApprovalDenied, by)
	if err != nil {
		return api.ApprovalInfo{}, err
	}
	d.tell(a, fmt.Sprintf("denied by %s: %s", by, a.Summary))
	return approvalInfo(a), nil
}

func (d *Daemon) decide(id int64, state, by string) (store.Approval, error) {
	if strings.TrimSpace(by) == "" {
		by = "unknown"
	}
	cur, ok, err := d.st.Approval(id)
	switch {
	case err != nil:
		return store.Approval{}, err
	case !ok:
		return store.Approval{}, fmt.Errorf("%w: #%d", api.ErrNoApproval, id)
	case actionHash(cur.Action, cur.Args) != cur.Hash:
		return store.Approval{}, fmt.Errorf("approval #%d does not match its hash; refusing", id)
	}
	a, err := d.st.Decide(id, state, by, time.Now())
	if errors.Is(err, store.ErrNotPending) {
		d.expire()
		return store.Approval{}, fmt.Errorf("%w: #%d is %s", api.ErrNotPending, id, d.stateOf(id))
	}
	return a, err
}

func (d *Daemon) stateOf(id int64) string {
	if a, ok, _ := d.st.Approval(id); ok {
		return a.State
	}
	return "gone"
}

func (d *Daemon) carryOut(ctx context.Context, a store.Approval) (string, error) {
	switch a.Action {
	case bot.ActionAddRoutine:
		var req api.RoutineRequest
		if err := json.Unmarshal([]byte(a.Args), &req); err != nil {
			return "", err
		}
		info, err := d.AddRoutine(ctx, req)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("routine #%d added; it first fires %s", info.ID, info.Next.Local().Format("2006-01-02 15:04")), nil
	case bot.ActionCancelRoutine:
		var c cancelArgs
		if err := json.Unmarshal([]byte(a.Args), &c); err != nil {
			return "", err
		}
		return fmt.Sprintf("routine #%d deleted", c.ID), d.DeleteRoutine(ctx, c.ID)
	case bot.ActionAddRepo:
		var req bot.RepoRequest
		if err := json.Unmarshal([]byte(a.Args), &req); err != nil {
			return "", err
		}
		return d.addRepo(a, req)
	case bot.ActionToolCall:
		var c toolCallArgs
		if err := json.Unmarshal([]byte(a.Args), &c); err != nil {
			return "", err
		}
		b, ok := d.Lookup(a.Bot)
		if !ok {
			return "", fmt.Errorf("%w: %q", api.ErrUnknownBot, a.Bot)
		}
		return b.RunApproved(ctx, c.Tool, c.Args)
	}
	var def config.Bot
	if err := json.Unmarshal([]byte(a.Args), &def); err != nil {
		return "", err
	}
	switch a.Action {
	case bot.ActionCreate:
		return "bot " + def.Name + " created and started", d.CreateBot(ctx, def)
	case bot.ActionUpdate:
		return "bot " + def.Name + " changed and restarted", d.UpdateBot(ctx, def)
	case bot.ActionDelete:
		return "bot " + def.Name + " deleted", d.DeleteBot(ctx, def.Name)
	}
	return "", fmt.Errorf("unknown action %q", a.Action)
}

// tell mails an approval's outcome to the bot that asked, if it still exists.
func (d *Daemon) tell(a store.Approval, text string) {
	if b, ok := d.Lookup(a.Bot); ok {
		_, _ = b.Deliver(fmt.Sprintf("approval #%d", a.ID), text)
	}
}

func (d *Daemon) expire() {
	expired, err := d.st.ExpireApprovals(time.Now())
	if err != nil {
		return
	}
	for _, a := range expired {
		d.tell(a, "expired without a decision, so it is denied: "+a.Summary)
	}
}

func (d *Daemon) expireLoop(ctx context.Context) {
	t := time.NewTicker(expireEvery)
	defer t.Stop()
	for {
		d.expire()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// describe is a one-line account of a definition for the approval summary.
func describe(b config.Bot) string {
	parts := []string{"model " + b.Model, "capabilities: " + b.Capabilities.String()}
	if b.Spend.Daily > 0 || b.Spend.Monthly > 0 {
		parts = append(parts, fmt.Sprintf("spend $%.2f/day $%.2f/month", b.Spend.Daily, b.Spend.Monthly))
	} else {
		parts = append(parts, "no spend cap")
	}
	return strings.Join(parts, ", ")
}

// changed lists the fields that differ between two definitions.
func changed(old, new config.Bot) string {
	flat := func(b config.Bot) map[string]string {
		m := map[string]string{}
		raw, _ := json.Marshal(b)
		var top map[string]any
		_ = json.Unmarshal(raw, &top)
		var walk func(prefix string, v any)
		walk = func(prefix string, v any) {
			if obj, ok := v.(map[string]any); ok {
				for k, sub := range obj {
					walk(strings.TrimPrefix(prefix+"."+k, "."), sub)
				}
				return
			}
			s, _ := json.Marshal(v)
			m[prefix] = string(s)
		}
		walk("", top)
		return m
	}
	a, b := flat(old), flat(new)
	var out []string
	for _, k := range sortedKeys(a, b) {
		switch {
		case a[k] == b[k]:
		case k == "prompt_text":
			out = append(out, "prompt changed")
		default:
			out = append(out, fmt.Sprintf("%s %s -> %s", k, orNone(a[k]), orNone(b[k])))
		}
	}
	return strings.Join(out, "; ")
}

func orNone(s string) string {
	if s == "" || s == "null" {
		return "(none)"
	}
	return s
}

func sortedKeys(ms ...map[string]string) []string {
	seen := map[string]bool{}
	var keys []string
	for _, m := range ms {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	slices.Sort(keys)
	return keys
}
