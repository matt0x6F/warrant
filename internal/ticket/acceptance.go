package ticket

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// AcceptanceTestFailure is returned when the acceptance test fails on submit.
type AcceptanceTestFailure struct {
	Stdout string
	Stderr string
}

func (e *AcceptanceTestFailure) Error() string {
	msg := "acceptance test failed"
	if e.Stderr != "" {
		msg += ": " + strings.TrimSpace(e.Stderr)
	} else if e.Stdout != "" {
		msg += ": " + strings.TrimSpace(e.Stdout)
	}
	return msg
}

// AcceptanceRunner runs the objective's acceptance_test (e.g. shell command) and reports pass/fail.
type AcceptanceRunner interface {
	Run(ctx context.Context, script string) (passed bool, stdout, stderr string, err error)
}

// ShellAcceptanceRunner runs the script with sh -c.
type ShellAcceptanceRunner struct {
	WorkDir string
}

func (r *ShellAcceptanceRunner) Run(ctx context.Context, script string) (passed bool, stdout, stderr string, err error) {
	if script == "" {
		return true, "", "", nil
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	if r.WorkDir != "" {
		cmd.Dir = r.WorkDir
	}
	out, err := cmd.CombinedOutput()
	stdout = string(out)
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, stdout, "", &AcceptanceTestFailure{Stdout: stdout, Stderr: ""}
		}
		return false, stdout, "", fmt.Errorf("acceptance test: %w", err)
	}
	return true, stdout, "", nil
}
