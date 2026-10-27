# Plan: terminal emulator and multiplexer

`internal/term` is Misa's VT emulator. Every pane, `misa --gui` and the web terminal draw from it, and every
byte it parses comes from a program the user ran, so it is treated as hostile input.

## Done

- Bounded parsing: CSI parameters saturate at 65535, at most 32 parameters and 16 sub-parameters, at most 8
  combining marks per cell, a bounded reply queue, a bounded style cache.
- DCS, APC, PM and SOS strings have their own ignored state; only real DA queries are answered.
- DEC 2026 synchronized output; scrollback with recycled rows; pointer-free cells.
- Copy mode with scrollback, the `ctrl+w` prefix, mouse forwarding, bracketed paste, native execution
  (`!cmd` gives a program the whole host terminal), follow-tail that stops when you scroll up.
- Session cleanup: on Linux every process group in the PTY's session is killed on close; elsewhere the leader's
  group and the foreground group are.
- Hostile-output tests (`hostile_test.go`) and a throughput benchmark (`go test -bench ProcessOutput`).
- `FuzzProcessOutput`: where output is split never changes what is drawn; around a resize there is no panic,
  the cursor stays on screen and scrollback stays bounded. Run it with
  `go test -fuzz FuzzProcessOutput ./internal/term/`.

## Next

1. **TERM and COLORTERM.** Children get `TERM=xterm-256color` and no `COLORTERM`, though the emulator keeps
   RGB colours. Set `COLORTERM=truecolor` when the frontend can show it (the GUI and web always can; a pane
   depends on the host terminal's colour profile). Keep `xterm-256color` unless a shipped terminfo entry
   says more.
2. **Capability queries.** Decide whether to answer XTGETTCAP (`DCS + q`) and DECRQSS for the few capabilities
   programs ask about (`RGB`, `Tc`, `Smulx`). Answering nothing is safe; answering wrongly is not.
3. **Background jobs off Linux.** Without `/proc` a PTY session's background process groups survive pane close
   on OpenBSD and FreeBSD. Track the process groups Misa itself starts (job control knows them) and kill those.
4. **Colour fidelity checks.** Indexed colours stay indexed until rendering. Add rendering tests under more than
   one host colour profile (truecolor, 256, 16) so a downgrade never changes which palette slot is used.
5. **Geometry audit.** `forceFrame()` renders at `w-1` columns to avoid host auto-wrap. Check whether the last
   column can be used again (DECAWM off in the root view) without bringing the wrap corruption back.

## Rules

- Keep `Cell` pointer-free (a test enforces it) and steady output allocation-free.
- Every new sequence gets a bound on what it can allocate and a test in `hostile_test.go`.
- Nothing a program prints may reach another pane or the host terminal except through the frontends' explicit
  paths (OSC title, OSC 52 behind `gui.yaml`'s `clipboard_write`).
- Run `go test -race ./internal/term/ ./internal/webui/ ./internal/gui/` after changing session or snapshot code.
