package asdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// StreamFunc receives one line of command output as it is produced.
type StreamFunc func(stream, line string)

// Result is the outcome of an asdf invocation.
type Result struct {
	Args     []string `json:"args"`
	Dir      string   `json:"dir"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
	ExitCode int      `json:"exitCode"`
	Duration int64    `json:"durationMs"`
}

// Runner executes asdf. The interface exists so a WSL bridge could be added
// later; only the local implementation ships today.
type Runner interface {
	Run(ctx context.Context, dir string, extraEnv []string, stream StreamFunc, args ...string) (Result, error)
	Command(args ...string) string
	BinaryPath() string
	DataDir() string
}

const captureLimit = 8 << 20 // 8 MiB is ample for any command output we parse.

// LocalRunner runs the asdf binary on the host.
type LocalRunner struct {
	binaryPath string
	dataDir    string
}

func NewLocalRunner(binaryPath, dataDir string) *LocalRunner {
	return &LocalRunner{binaryPath: binaryPath, dataDir: dataDir}
}

func (r *LocalRunner) BinaryPath() string { return r.binaryPath }
func (r *LocalRunner) DataDir() string    { return r.dataDir }

// Command renders the command without executing it.
func (r *LocalRunner) Command(args ...string) string {
	return r.binaryPath + " " + QuoteCommand(args)
}

func (r *LocalRunner) Run(ctx context.Context, dir string, extraEnv []string, stream StreamFunc, args ...string) (Result, error) {
	result := Result{Args: args, Dir: dir}
	started := time.Now()
	defer func() { result.Duration = time.Since(started).Milliseconds() }()

	if r.binaryPath == "" {
		return result, errors.New("asdf binary is not configured")
	}

	cmd := exec.CommandContext(ctx, r.binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = append(BuildEnv(r.binaryPath, r.dataDir), extraEnv...)
	configureProcessGroup(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, fmt.Errorf("open stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return result, fmt.Errorf("open stderr: %w", err)
	}

	stdoutBuf := &limitedBuffer{limit: captureLimit}
	stderrBuf := &limitedBuffer{limit: captureLimit}

	if err := cmd.Start(); err != nil {
		return result, fmt.Errorf("start asdf: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		readLines(stdout, "stdout", stdoutBuf, stream)
	}()
	go func() {
		defer wg.Done()
		readLines(stderr, "stderr", stderrBuf, stream)
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	result.ExitCode = cmd.ProcessState.ExitCode()

	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if waitErr != nil {
		if result.ExitCode < 0 {
			result.ExitCode = 1
		}
		return result, &ExitError{Result: result, Err: waitErr}
	}
	return result, nil
}

// ExitError reports a non-zero asdf exit.
type ExitError struct {
	Result Result
	Err    error
}

func (e *ExitError) Error() string {
	detail := strings.TrimSpace(e.Result.Stderr)
	if detail == "" {
		detail = strings.TrimSpace(e.Result.Stdout)
	}
	if detail == "" {
		return fmt.Sprintf("asdf exited with code %d", e.Result.ExitCode)
	}
	return fmt.Sprintf("asdf exited with code %d: %s", e.Result.ExitCode, truncate(detail, 800))
}

func (e *ExitError) Unwrap() error { return e.Err }

func readLines(reader interface{ Read([]byte) (int, error) }, streamName string, capture *limitedBuffer, stream StreamFunc) {
	buf := make([]byte, 32*1024)
	var pending strings.Builder
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			capture.WriteString(chunk)
			pending.WriteString(chunk)
			for {
				text := pending.String()
				idx := strings.IndexByte(text, '\n')
				if idx < 0 {
					break
				}
				line := strings.TrimSuffix(text[:idx], "\r")
				pending.Reset()
				pending.WriteString(text[idx+1:])
				if stream != nil {
					stream(streamName, line)
				}
			}
		}
		if err != nil {
			break
		}
	}
	if rest := pending.String(); rest != "" {
		if stream != nil {
			stream(streamName, strings.TrimSuffix(rest, "\r"))
		}
	}
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) WriteString(s string) {
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return
	}
	if len(s) > remaining {
		b.buf.WriteString(s[:remaining])
		b.truncated = true
		return
	}
	b.buf.WriteString(s)
}

func (b *limitedBuffer) String() string {
	if b.truncated {
		return b.buf.String() + "\n[output truncated]"
	}
	return b.buf.String()
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}
