package ops

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AshutoshSajan/asdf-gui/internal/asdf"
)

type fakeRunner struct {
	mu        sync.Mutex
	calls     [][]string
	active    int
	maxActive int
	delay     time.Duration
	block     chan struct{}
}

func (f *fakeRunner) Command(args ...string) string { return "asdf " + strings.Join(args, " ") }
func (f *fakeRunner) BinaryPath() string            { return "/fake/asdf" }
func (f *fakeRunner) DataDir() string               { return "/fake/data" }

func (f *fakeRunner) Run(ctx context.Context, dir string, env []string, stream asdf.StreamFunc, args ...string) (asdf.Result, error) {
	f.mu.Lock()
	f.calls = append(f.calls, append([]string(nil), args...))
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()

	if stream != nil {
		stream("stdout", "running "+strings.Join(args, " "))
	}
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return asdf.Result{Args: args, ExitCode: 130}, ctx.Err()
		}
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return asdf.Result{Args: args, ExitCode: 130}, ctx.Err()
		}
	}
	if len(args) > 0 && args[0] == "fail" {
		return asdf.Result{Args: args, ExitCode: 2, Stderr: "boom"}, &asdf.ExitError{
			Result: asdf.Result{Args: args, ExitCode: 2, Stderr: "boom"}, Err: errors.New("exit 2"),
		}
	}
	return asdf.Result{Args: args, ExitCode: 0, Stdout: "ok"}, nil
}

func newTestQueue(t *testing.T, runner *fakeRunner) (*Queue, <-chan string) {
	t.Helper()
	events := make(chan string, 128)
	queue := NewQueue(func() (asdf.Runner, error) { return runner, nil }, t.TempDir())
	queue.SetEmitter(func(name string, _ any) {
		select {
		case events <- name:
		default:
		}
	})
	t.Cleanup(queue.Close)
	return queue, events
}

func waitEvent(t *testing.T, events <-chan string, want string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case event := <-events:
			if event == want {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", want)
		}
	}
}

func TestQueueSerializesOperations(t *testing.T) {
	runner := &fakeRunner{delay: 20 * time.Millisecond}
	queue, events := newTestQueue(t, runner)

	first, err := queue.Enqueue(Request{Kind: "install", Title: "first", Args: []string{"install", "nodejs", "22.0.0"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := queue.Enqueue(Request{Kind: "install", Title: "second", Args: []string{"install", "ruby", "3.3.0"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != StatusQueued || second.Status != StatusQueued {
		t.Fatalf("expected queued, got %s and %s", first.Status, second.Status)
	}

	waitEvent(t, events, EventFinished)
	waitEvent(t, events, EventFinished)

	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.maxActive != 1 {
		t.Fatalf("expected serial execution, max active was %d", runner.maxActive)
	}
	if len(runner.calls) != 2 || runner.calls[0][0] != "install" || runner.calls[1][0] != "install" {
		t.Fatalf("unexpected calls: %+v", runner.calls)
	}

	list := queue.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(list))
	}
	for _, op := range list {
		if op.Status != StatusSucceeded {
			t.Fatalf("unexpected status: %+v", op)
		}
		if _, err := os.Stat(op.LogPath); err != nil {
			t.Fatalf("log file missing: %v", err)
		}
	}
}

func TestQueueReportsFailure(t *testing.T) {
	runner := &fakeRunner{}
	queue, events := newTestQueue(t, runner)
	_, err := queue.Enqueue(Request{Kind: "install", Title: "fail", Args: []string{"fail", "x"}})
	if err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, EventFinished)
	list := queue.List()
	if len(list) != 1 || list[0].Status != StatusFailed {
		t.Fatalf("expected failed operation: %+v", list)
	}
	if !strings.Contains(list[0].Error, "boom") {
		t.Fatalf("unexpected error: %s", list[0].Error)
	}
}

func TestQueueCancelRunning(t *testing.T) {
	block := make(chan struct{})
	runner := &fakeRunner{block: block}
	queue, events := newTestQueue(t, runner)

	op, err := queue.Enqueue(Request{Kind: "install", Title: "slow", Args: []string{"install", "nodejs"}})
	if err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, EventStarted)
	if _, err := queue.Cancel(op.ID); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, EventFinished)

	list := queue.List()
	if len(list) != 1 || list[0].Status != StatusCanceled {
		t.Fatalf("expected canceled operation: %+v", list)
	}
	close(block)
}

func TestQueueCancelQueued(t *testing.T) {
	block := make(chan struct{})
	runner := &fakeRunner{block: block}
	queue, events := newTestQueue(t, runner)

	first, err := queue.Enqueue(Request{Kind: "install", Title: "first", Args: []string{"install", "nodejs"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := queue.Enqueue(Request{Kind: "install", Title: "second", Args: []string{"install", "ruby"}})
	if err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, EventStarted)
	if _, err := queue.Cancel(second.ID); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, EventFinished)
	if _, err := queue.Cancel(first.ID); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, EventFinished)
	close(block)

	list := queue.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(list))
	}
	statuses := map[string]Status{}
	for _, op := range list {
		statuses[op.ID] = op.Status
	}
	if statuses[first.ID] != StatusCanceled || statuses[second.ID] != StatusCanceled {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}
}

func TestQueueClearFinished(t *testing.T) {
	runner := &fakeRunner{}
	queue, events := newTestQueue(t, runner)
	if _, err := queue.Enqueue(Request{Kind: "install", Title: "one", Args: []string{"install", "nodejs"}}); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, EventFinished)
	if removed := queue.ClearFinished(); removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if len(queue.List()) != 0 {
		t.Fatalf("expected empty list")
	}
}

func TestQueueDetailContainsOutput(t *testing.T) {
	runner := &fakeRunner{}
	queue, events := newTestQueue(t, runner)
	op, err := queue.Enqueue(Request{Kind: "install", Title: "one", Args: []string{"install", "nodejs"}})
	if err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, EventFinished)
	detail, err := queue.Get(op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Output) == 0 {
		t.Fatal("expected buffered output")
	}
	if detail.Output[0].Stream != "stdout" {
		t.Fatalf("unexpected stream: %+v", detail.Output[0])
	}
}
