package vi

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

type shellRunResult struct {
	stdout, stderr string
	exitCode       int
	err            error
}

func runShell(ctx context.Context, command string, input []byte, dir string, env []string) shellRunResult {
	cmd := exec.CommandContext(ctx, shellExecutable(), "-c", command)
	cmd.Dir, cmd.Env, cmd.Stdin = dir, env, bytes.NewReader(input)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	configureShellProcess(cmd)
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	code := 0
	if err != nil {
		code = -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
	}
	return shellRunResult{stdout: out.String(), stderr: stderr.String(), exitCode: code, err: err}
}
