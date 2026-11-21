package exec

import (
	"context"
	"fmt"
	osexec "os/exec"
)

// OSRunner implements domain.CommandRunner using os/exec.
type OSRunner struct{}

// NewOSRunner creates a new OSRunner.
func NewOSRunner() *OSRunner {
	return &OSRunner{}
}

// LookPath checks whether an executable exists on PATH.
func (r *OSRunner) LookPath(file string) (string, error) {
	return osexec.LookPath(file)
}

// Run executes a command inside dir and returns combined stdout/stderr output.
func (r *OSRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := osexec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("command %q failed: %w\n%s", name, err, string(out))
	}
	return out, nil
}
