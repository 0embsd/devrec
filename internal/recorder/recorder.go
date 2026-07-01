package recorder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Recorder wraps the script command with signal-safe lifecycle.
type Recorder struct {
	shell      string
	logFile    string
	timingFile string
}

// NewRecorder creates a terminal recorder.
func NewRecorder(shell, logFile, timingFile string) *Recorder {
	if shell == "" {
		shell = resolveShell()
	}
	return &Recorder{
		shell:      shell,
		logFile:    logFile,
		timingFile: timingFile,
	}
}

// Start runs script in the foreground. It blocks until script exits.
// The provided ctx is cancelled by signal handlers (SIGINT/SIGTERM/SIGHUP),
// which causes Start to send SIGTERM to the script child, wait, then return.
func (r *Recorder) Start(ctx context.Context) (scriptPID int, err error) {
	scriptPath, err := exec.LookPath("script")
	if err != nil {
		return 0, fmt.Errorf("script: utility not found; install util-linux (apt install util-linux)")
	}

	// script --quiet --timing=<timingFile> --command=<shell> <logFile>
	cmd := exec.CommandContext(ctx, scriptPath,
		"--quiet",
		"--timing="+r.timingFile,
		"--command="+r.shell,
		r.logFile,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start script: %w", err)
	}

	// script's PID is cmd.Process.Pid.
	// SIGTERM on this PID propagates to the child shell.
	scriptPID = cmd.Process.Pid

	// Wait for script to exit or ctx to be cancelled.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Context cancelled: send SIGTERM, wait, then SIGKILL if needed.
		cmd.Process.Signal(syscall.SIGTERM)

		// Wait up to 5 seconds for graceful shutdown.
		select {
		case <-done:
			return scriptPID, ctx.Err()
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			<-done
			return scriptPID, ctx.Err()
		}
	case err := <-done:
		return scriptPID, err
	}
}

// resolveShell returns the shell to use for recording.
func resolveShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "bash"
}

// GetScriptPID tries to find the PID of the script child process.
// Reads /proc/<pid>/task/<pid>/children to find the inner shell PID.
func GetScriptPID(scriptPID int) int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", scriptPID, scriptPID))
	if err != nil {
		return scriptPID // fallback: return the script PID itself
	}
	fields := strings.Fields(string(data))
	if len(fields) > 0 {
		if pid, err := strconv.Atoi(fields[0]); err == nil {
			return pid
		}
	}
	return scriptPID
}
