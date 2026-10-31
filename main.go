package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/builtinexec"
	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/jobleader"
	"github.com/AmadeusDE/Misa/internal/proceffect"
	"github.com/AmadeusDE/Misa/internal/ui"
	"golang.org/x/term"
)

func main() {
	_ = proceffect.EnableSelfHelper()
	if len(os.Args) > 1 && os.Args[1] == proceffect.Flag {
		os.Exit(builtinexec.ProcessEffect(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == jobleader.Flag {
		os.Exit(jobleader.Run(os.Args[2:]))
	}
	name := filepath.Base(os.Args[0])

	if name == "busybox" {
		if len(os.Args) > 1 {
			if os.Args[1] == "--list" || os.Args[1] == "list" || os.Args[1] == "--list-builtins" {
				for _, n := range builtins.Names() {
					fmt.Println(n)
				}
				os.Exit(0)
			}
			if _, ok := builtins.Lookup(os.Args[1]); ok {
				os.Exit(runStandalone(os.Args[1], os.Args[2:]))
			}
		}
		fmt.Println("BusyBox v1.36.1 (Misa multicall binary)")
		os.Exit(0)
	}

	switch name {
	case "misa-gui":
		os.Exit(startGUI(os.Args[1:]))
	case "misa-ssh":
		os.Exit(startSSH(os.Args[1:]))
	}

	if name != "misa" {
		if _, ok := builtins.Lookup(name); ok {
			os.Exit(runStandalone(name, os.Args[1:]))
		}
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--install":
			os.Exit(builtins.InstallLinks(os.Args[0], os.Args[2:], os.Stderr))
		case "--gui":
			os.Exit(startGUI(os.Args[2:]))
		case "--ssh":
			os.Exit(startSSH(os.Args[2:]))
		case "--web":
			os.Exit(runWeb(os.Args[2:]))
		case "--web-admin":
			os.Exit(runWebAdmin(os.Args[2:]))
		case "--list", "--list-builtins":
			for _, n := range builtins.Names() {
				fmt.Println(n)
			}
			return
		}
		if _, ok := builtins.Lookup(os.Args[1]); ok {
			os.Exit(runStandalone(os.Args[1], os.Args[2:]))
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "misa:", err)
		os.Exit(1)
	}

	if exe, err := os.Executable(); err == nil {
		ui.BuiltinExecutable = exe
	}
	model := ui.New(dir)
	_, err = tea.NewProgram(model).Run()
	model.Shutdown()
	if err != nil {
		fmt.Fprintln(os.Stderr, "misa:", err)
		os.Exit(1)
	}
}

// startGUI opens Misa in its own Wayland window; with args it runs that
// command there instead.
func startGUI(args []string) int {
	dir, err := os.Getwd()
	if err != nil {
		dir = "/"
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "misa:", err)
		return 1
	}
	argv, title := guiCommand(exe, args, exec.LookPath)
	return runGUI(dir, argv, title)
}

// startSSH opens the window as a plain ssh client: sshc and nothing else.
func startSSH(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: misa --ssh [sshc options] destination [command]")
		return 2
	}
	return startGUI(append([]string{"sshc"}, args...))
}

// guiCommand is what the window runs: Misa itself, a Misa builtin through the
// multicall binary when no program of that name is installed, or the command.
func guiCommand(exe string, args []string, lookPath func(string) (string, error)) (argv []string, title string) {
	if len(args) == 0 {
		return []string{exe}, "Misa"
	}
	title = strings.Join(args, " ")
	if _, ok := builtins.Lookup(args[0]); ok {
		if _, err := lookPath(args[0]); err != nil {
			return append([]string{exe}, args...), title
		}
	}
	return args, title
}

func runStandalone(name string, args []string) int {
	signal.Ignore(syscall.SIGPIPE)
	jobleader.SuspendOnCtrlZ()

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, name+":", err)
		return 1
	}

	cancelCtx, stopSignals := standaloneBuiltinContext(name)
	defer stopSignals()
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))
	ctx := &builtins.Context{
		Ctx:       cancelCtx,
		Exec:      builtinexec.For(name),
		StdinTTY:  term.IsTerminal(int(os.Stdin.Fd())),
		StdoutTTY: term.IsTerminal(int(os.Stdout.Fd())),
		StderrTTY: term.IsTerminal(int(os.Stderr.Fd())),
		Height:    height,
		Dir:       dir,
		Args:      args,
		Width:     width,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Stdin:     os.Stdin,
	}

	return builtins.Run(cancelCtx, name, ctx)
}
