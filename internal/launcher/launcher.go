package launcher

import (
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

type Finished struct {
	Err error
}

func Command(dir string, argv ...string) *exec.Cmd {
	if len(argv) == 0 {
		return nil
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	return cmd
}

func Run(dir string, argv ...string) tea.Cmd {
	cmd := Command(dir, argv...)
	if cmd == nil {
		return nil
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return Finished{Err: err}
	})
}

func UserShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}
