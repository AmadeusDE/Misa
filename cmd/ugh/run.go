// Headless chat: `ugh -p "..."`. Wiring for the OpenCode Go provider (M1),
// permission gate (M2) and resumable sessions (M3); the default (no -p)
// launches the TUI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/AmadeusDE/Misa/internal/ugh/agent/host"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/agent"
	"github.com/AmadeusDE/Misa/internal/ugh/config"
	"github.com/AmadeusDE/Misa/internal/ugh/perm"
	"github.com/AmadeusDE/Misa/internal/ugh/policy"
	"github.com/AmadeusDE/Misa/internal/ugh/provider"
	"github.com/AmadeusDE/Misa/internal/ugh/ughtui"
	"golang.org/x/term"
)

// defaultModelRef honors an explicit configured default, otherwise the first
// model of the policy's default root scope (OpenRouter MiMo Pro when an
// OpenRouter key exists). The saved /scoped-models list does not pick it. No
// credentials at all means no default rather than a route that cannot work.
func defaultModelRef() string {
	if cfg, err := config.Load(); err == nil && cfg.Models.Default != "" {
		return cfg.Models.Default
	}
	if refs := provider.DefaultRootScope(); len(refs) > 0 {
		return refs[0]
	}
	return ""
}

// chatCmd parses the default (no-subcommand) invocation.
func chatCmd(args []string) error {
	fs := flag.NewFlagSet("ugh", flag.ContinueOnError)
	prompt := fs.String("p", "", "run one prompt headless and exit")
	model := fs.String("m", "", "model ref: provider/model (default: config models.default, else OpenRouter MiMo Pro, else OpenCode Go)")
	thinking := fs.String("thinking", "", "thinking level: off|minimal|low|medium|high|xhigh|max (default high; clamped to what the model supports)")
	system := fs.String("system", "", "system prompt (default: terse coding agent)")
	cwd := fs.String("cwd", "", "working directory (default: current)")
	mode := fs.String("mode", "", "permission mode: manual|classify|auto (default: config, classify)")
	resume := fs.Bool("resume", false, "continue the most recent session for this cwd")
	maxTurns := fs.Int("max-turns", 0, "turn budget for the root agent (default: config agent.max_turns_root)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	path, err := config.Path()
	if err != nil {
		return err
	}
	warnings, err := policy.MigrateConfig(&cfg, path)
	if err != nil {
		return fmt.Errorf("configuration migration: %w", err)
	}
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}
	// Context windows and thinking levels come from the cached listings;
	// keep them current in the background (at most daily).
	go provider.RefreshModelsIfStale(context.Background(), 24*time.Hour)
	if *model == "" {
		*model = defaultModelRef()
	}
	if *model == "" && !*resume {
		return fmt.Errorf("no eligible root models in the current scope; log in, configure a scope, or choose an explicit -m provider/model")
	}
	dir := *cwd
	if dir == "" {
		d, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = d
	}
	sys := *system
	if sys == "" {
		sys = agent.DefaultSystem
	}
	switch {
	case *prompt != "":
		return headless(*prompt, *model, *thinking, sys, dir, *mode, *resume, *maxTurns)
	case fs.NArg() > 0:
		return fmt.Errorf("unknown argument %q (use -p \"...\" for headless, no args for the TUI)", fs.Arg(0))
	default:
		return runTUI(ughtui.Options{
			Model: *model, Thinking: *thinking, System: *system,
			Cwd: dir, Mode: *mode, Resume: *resume, MaxTurns: *maxTurns,
		})
	}
}

// headless runs one prompt through the permission gate and streams the
// transcript (assistant text on stdout, tool trace and prompts on stderr).
// With -resume, the previous session's history seeds the run and new
// messages append to the same JSONL file.
func headless(prompt, modelRef, thinking, system, cwd, mode string, resume bool, maxTurns int) error {
	broker := perm.NewBroker(os.Stdin, os.Stderr, stdinInteractive(), "")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	run, err := host.New(ctx, host.Options{
		Cwd: cwd, Model: modelRef, Thinking: thinking, System: system, Mode: mode,
		MaxTurns: maxTurns, Resume: resume, Prompter: broker,
		Out: os.Stdout, Trace: os.Stderr,
	})
	if err != nil {
		return err
	}
	defer run.Close()
	start := time.Now()
	final, err := run.Run(ctx, prompt)
	a := run.Agent
	var turnOut *agent.TurnOutError
	if errors.As(err, &turnOut) && !turnOut.Answered {
		fmt.Println(final) // the [TURN-OUT] report; an answered turn-out already streamed
	}
	if len(a.Denied) > 0 {
		fmt.Fprintf(os.Stderr, "[%d tool call(s) denied by the permission policy:\n- %s]\n", len(a.Denied), strings.Join(a.Denied, "\n- "))
	}
	if err != nil {
		return err
	}
	// Auto mode classifies in the background; give it a moment to land in
	// decisions.jsonl before the process exits.
	run.WaitBackground(6 * time.Second)
	fmt.Fprintf(os.Stderr, "[%s · %d in / %d out / %d cached · %s]\n",
		run.Model, a.Usage.In, a.Usage.Out, a.Usage.Cached, time.Since(start).Round(time.Millisecond))
	return nil
}

// stdinInteractive reports whether prompts may read answers from stdin: a
// terminal, or a pipe/file with lines coming (EOF falls back to the headless
// policy). A char device that is not a terminal (/dev/null) is headless.
func stdinInteractive() bool {
	st, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	if st.Mode()&os.ModeCharDevice != 0 {
		return term.IsTerminal(int(os.Stdin.Fd()))
	}
	return true
}
