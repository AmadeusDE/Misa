//go:build !unix

package vi

import "os/exec"

func shellExecutable() string { return "sh" }

func configureShellProcess(cmd *exec.Cmd) {}
