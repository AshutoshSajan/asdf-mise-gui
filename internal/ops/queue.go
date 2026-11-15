// Package ops serializes mutations of asdf's shared state into a FIFO queue,
// streams their output, and persists each run to a log file.
package ops

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/asdf"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCanceling Status = "canceling"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

const (
	EventQueued   = "operation:queued"
	EventStarted  = "operation:started"
	EventUpdated  = "operation:updated"
	EventOutput   = "operation:output"
	EventFinished = "operation:finished"
	EventState    = "state:changed"
)

// Operation is a single asdf invocation.
type Operation struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Command    string `json:"command"`
	Dir        string `json:"dir"`
	Status     Status `json:"status"`
	CreatedAt  string `json:"createdAt"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	ExitCode   int    `json:"exitCode"`
	Error      string `json:"error,omitempty"`
	LogPath    string `json:"logPath"`
}

// OutputLine is one line of streamed output.
type OutputLine struct {
	Stream string `json:"stream"`
	Text   string `json:"text"`
}

// OutputBatch is the payload of operation:output.
type OutputBatch struct {
	OperationID string       `json:"operationId"`
	Lines       []OutputLine `json:"lines"`
}

// Detail is an operation plus its in-memory tail.
type Detail struct {
	Operation Operation    `json:"operation"`
	Output    []OutputLine `json:"output"`
}

// Step is one command in a queued operation. Most requests have a single step;
// config writers use several so that a save is still serialized as one unit.
type Step struct {
	Args []string
	Env  []string
}

// Request describes an enqueued version manager command.
type Request struct {
	Kind  string
	Title string
	Dir   string
	Args  []string
	Env   []string
	Steps []Step
}

type RunnerFactory func() (asdf.Runner, error)
type Emitter func(name string, data any)

type entry struct {
	op     Operation
	output []OutputLine
	steps  []Step
	ctx    context.Context
	cancel context.CancelFunc
	runner asdf.Runner
}

// Queue runs one asdf mutation at a time.
type Queue struct {
	mu      sync.Mutex
	factory RunnerFactory
	emit    func(name string, data any)
	logDir  string
	ops     map[string]*entry
	order   []string
	pending []string
	closed  bool
	seq     int
	wake    chan struct{}
	rootCtx context.Context
	stop    context.CancelFunc
	worker  sync.WaitGroup

	onChange func()
}

const maxBufferedLines = 4000

func NewQueue(factory RunnerFactory, logDir string) *Queue {
	ctx, cancel := context.WithCancel(context.Background())
	q := &Queue{
		factory: factory,
		logDir:  logDir,
		ops:     map[string]*entry{},
		wake:    make(chan struct{}, 1),
		rootCtx: ctx,
		stop:    cancel,
	}
	q.worker.Add(1)
	go q.loop()
	return q
}

// SetEmitter installs the event sink after the Wails app exists.
func (q *Queue) SetEmitter(emit Emitter) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if emit == nil {
		q.emit = nil
		return
	}
	q.emit = func(name string, data any) { emit(name, data) }
}

// SetOnChange registers a callback fired whenever an operation finishes.
func (q *Queue) SetOnChange(fn func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.onChange = fn
}

func (q *Queue) Enqueue(request Request) (Operation, error) {
	steps := request.Steps
	if len(steps) == 0 && len(request.Args) > 0 {
		steps = []Step{{Args: request.Args, Env: request.Env}}
	}
	if len(steps) == 0 {
		return Operation{}, errors.New("no command to run")
	}
	for _, step := range steps {
		if len(step.Args) == 0 {
			return Operation{}, errors.New("no command to run")
		}
	}
	runner, err := q.factory()
	if err != nil {
		return Operation{}, err
	}
	if request.Dir != "" {
		if info, statErr := os.Stat(request.Dir); statErr != nil || !info.IsDir() {
			return Operation{}, fmt.Errorf("project directory does not exist: %s", request.Dir)
		}
	}

	if err := os.MkdirAll(q.logDir, 0o755); err != nil {
		return Operation{}, err
	}

	commands := make([]string, 0, len(steps))
	for _, step := range steps {
		commands = append(commands, runner.Command(step.Args...))
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return Operation{}, errors.New("the operation queue is shutting down")
	}
	q.seq++
	id := fmt.Sprintf("op-%d-%d", time.Now().Unix(), q.seq)
	now := time.Now().UTC().Format(time.RFC3339)
	op := Operation{
		ID:        id,
		Kind:      request.Kind,
		Title:     request.Title,
		Command:   strings.Join(commands, "\n"),
		Dir:       request.Dir,
		Status:    StatusQueued,
		CreatedAt: now,
		LogPath:   filepath.Join(q.logDir, id+".log"),
	}
	if op.Title == "" {
		op.Title = op.Command
	}
	ctx, cancel := context.WithCancel(q.rootCtx)
	copied := make([]Step, len(steps))
	for index, step := range steps {
		copied[index] = Step{
			Args: append([]string(nil), step.Args...),
			Env:  append([]string(nil), step.Env...),
		}
	}
	q.ops[id] = &entry{
		op:     op,
		steps:  copied,
		ctx:    ctx,
		cancel: cancel,
		runner: runner,
	}
	q.order = append(q.order, id)
	q.pending = append(q.pending, id)
	q.emitLocked(EventQueued, op)
	q.notifyLocked()
	return op, nil
}

func (q *Queue) loop() {
	defer q.worker.Done()
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return
		}
		var next string
		for len(q.pending) > 0 {
			candidate := q.pending[0]
			q.pending = q.pending[1:]
			item, ok := q.ops[candidate]
			if !ok || item.op.Status != StatusQueued {
				continue
			}
			next = candidate
			break
		}
		q.mu.Unlock()

		if next == "" {
			select {
			case <-q.wake:
				continue
			case <-q.rootCtx.Done():
				return
			}
		}
		q.run(next)
	}
}

func (q *Queue) run(id string) {
	q.mu.Lock()
	item, ok := q.ops[id]
	if !ok || item.op.Status != StatusQueued {
		q.mu.Unlock()
		return
	}
	if item.ctx.Err() != nil {
		q.finishLocked(item, StatusCanceled, 0, "canceled before start")
		q.mu.Unlock()
		q.dispatchFinished(item)
		return
	}
	item.op.Status = StatusRunning
	item.op.StartedAt = time.Now().UTC().Format(time.RFC3339)
	started := item.op
	command := item.op.Command
	dir := item.op.Dir
	runner := item.runner
	ctx := item.ctx
	steps := item.steps
	logPath := item.op.LogPath
	q.mu.Unlock()

	q.emitEvent(EventStarted, started)

	logFile, err := os.Create(logPath)
	if err != nil {
		q.complete(id, asdf.Result{}, fmt.Errorf("create log file: %w", err))
		return
	}
	defer logFile.Close()
	logWriter := bufio.NewWriter(logFile)
	fmt.Fprintf(logWriter, "$ %s\n# dir: %s\n# started: %s\n\n", command, displayDir(dir), started.StartedAt)
	logWriter.Flush()

	stream := make(chan OutputLine, 1024)
	pumpDone := make(chan struct{})
	go q.pump(id, stream, logWriter, pumpDone)

	var result asdf.Result
	var runErr error
	// Step markers go through the pump instead of writing to logWriter here:
	// the pump owns the writer, and routing markers through its channel keeps
	// the log in step order without concurrent writes to bufio.Writer.
	logMeta := func(text string) {
		select {
		case stream <- OutputLine{Stream: "meta", Text: text}:
		case <-ctx.Done():
		}
	}
	for index, step := range steps {
		if err := ctx.Err(); err != nil {
			runErr = err
			break
		}
		if len(steps) > 1 {
			logMeta(fmt.Sprintf("# step %d/%d\n$ %s\n\n", index+1, len(steps), runner.Command(step.Args...)))
		}
		result, runErr = runner.Run(ctx, dir, step.Env, func(streamName, line string) {
			select {
			case stream <- OutputLine{Stream: streamName, Text: line}:
			case <-ctx.Done():
			}
		}, step.Args...)
		logMeta(fmt.Sprintf("\n# step exit: %d\n", result.ExitCode))
		if runErr != nil {
			break
		}
	}

	close(stream)
	<-pumpDone
	fmt.Fprintf(logWriter, "\n# finished: %s\n# exit: %d\n", time.Now().UTC().Format(time.RFC3339), result.ExitCode)
	logWriter.Flush()

	q.complete(id, result, runErr)
}

func (q *Queue) pump(id string, stream <-chan OutputLine, logWriter *bufio.Writer, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(60 * time.Millisecond)
	defer ticker.Stop()

	var batch []OutputLine
	flush := func() {
		if len(batch) == 0 {
			return
		}
		for _, line := range batch {
			if line.Stream == "stderr" {
				fmt.Fprintf(logWriter, "[stderr] %s\n", line.Text)
			} else {
				fmt.Fprintln(logWriter, line.Text)
			}
		}
		logWriter.Flush()

		q.mu.Lock()
		if item, ok := q.ops[id]; ok {
			item.output = append(item.output, batch...)
			if len(item.output) > maxBufferedLines {
				item.output = append([]OutputLine(nil), item.output[len(item.output)-maxBufferedLines:]...)
			}
		}
		q.mu.Unlock()

		q.emitEvent(EventOutput, OutputBatch{OperationID: id, Lines: batch})
		batch = nil
	}

	for {
		select {
		case line, ok := <-stream:
			if !ok {
				flush()
				return
			}
			if line.Stream == "meta" {
				// Internal markers are written raw after earlier output so the
				// log stays ordered, but they are never shown in the UI.
				flush()
				logWriter.WriteString(line.Text)
				logWriter.Flush()
				continue
			}
			batch = append(batch, line)
			if len(batch) >= 200 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (q *Queue) complete(id string, result asdf.Result, runErr error) {
	q.mu.Lock()
	item, ok := q.ops[id]
	if !ok {
		q.mu.Unlock()
		return
	}
	status := StatusSucceeded
	message := ""
	if errors.Is(item.ctx.Err(), context.Canceled) {
		status = StatusCanceled
		message = "canceled"
	} else if runErr != nil {
		status = StatusFailed
		message = runErr.Error()
		if exitErr := new(asdf.ExitError); errors.As(runErr, &exitErr) {
			message = exitErr.Error()
		}
	}
	q.finishLocked(item, status, result.ExitCode, message)
	snapshot := item.op
	q.mu.Unlock()

	q.emitEvent(EventFinished, snapshot)
	q.dispatchChange()
	q.notify()
}

func (q *Queue) finishLocked(item *entry, status Status, exitCode int, message string) {
	item.op.Status = status
	item.op.ExitCode = exitCode
	item.op.Error = message
	item.op.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	item.cancel()
}

// Cancel asks an operation to stop. Queued operations are removed immediately.
func (q *Queue) Cancel(id string) (Operation, error) {
	q.mu.Lock()
	item, ok := q.ops[id]
	if !ok {
		q.mu.Unlock()
		return Operation{}, fmt.Errorf("operation %s not found", id)
	}
	switch item.op.Status {
	case StatusQueued:
		item.cancel()
		q.removePendingLocked(id)
		q.finishLocked(item, StatusCanceled, 0, "canceled before start")
		snapshot := item.op
		q.mu.Unlock()
		q.emitEvent(EventFinished, snapshot)
		q.dispatchChange()
		return snapshot, nil
	case StatusRunning:
		item.op.Status = StatusCanceling
		item.cancel()
		snapshot := item.op
		q.mu.Unlock()
		q.emitEvent(EventUpdated, snapshot)
		return snapshot, nil
	default:
		snapshot := item.op
		q.mu.Unlock()
		return snapshot, fmt.Errorf("operation is already %s", snapshot.Status)
	}
}

func (q *Queue) removePendingLocked(id string) {
	filtered := q.pending[:0]
	for _, candidate := range q.pending {
		if candidate != id {
			filtered = append(filtered, candidate)
		}
	}
	q.pending = filtered
}

// List returns newest first.
func (q *Queue) List() []Operation {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Operation, 0, len(q.order))
	for i := len(q.order) - 1; i >= 0; i-- {
		if item, ok := q.ops[q.order[i]]; ok {
			out = append(out, item.op)
		}
	}
	return out
}

func (q *Queue) Get(id string) (Detail, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	item, ok := q.ops[id]
	if !ok {
		return Detail{}, fmt.Errorf("operation %s not found", id)
	}
	return Detail{Operation: item.op, Output: append([]OutputLine(nil), item.output...)}, nil
}

// ClearFinished forgets completed operations. Log files stay on disk.
func (q *Queue) ClearFinished() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	kept := q.order[:0]
	removed := 0
	for _, id := range q.order {
		item, ok := q.ops[id]
		if !ok {
			continue
		}
		switch item.op.Status {
		case StatusSucceeded, StatusFailed, StatusCanceled:
			delete(q.ops, id)
			removed++
		default:
			kept = append(kept, id)
		}
	}
	q.order = kept
	return removed
}

// RunningCount reports queued + active operations.
func (q *Queue) RunningCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	count := 0
	for _, item := range q.ops {
		if item.op.Status != StatusSucceeded && item.op.Status != StatusFailed && item.op.Status != StatusCanceled {
			count++
		}
	}
	return count
}

func (q *Queue) Close() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	q.mu.Unlock()
	q.stop()
	q.worker.Wait()
}

func (q *Queue) notifyLocked() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *Queue) notify() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.notifyLocked()
}

func (q *Queue) emitEvent(name string, data any) {
	q.mu.Lock()
	emit := q.emit
	q.mu.Unlock()
	if emit != nil {
		emit(name, data)
	}
}

func (q *Queue) emitLocked(name string, data any) {
	if q.emit != nil {
		q.emit(name, data)
	}
}

func (q *Queue) dispatchFinished(item *entry) {
	snapshot := item.op
	q.emitEvent(EventFinished, snapshot)
	q.dispatchChange()
}

func (q *Queue) dispatchChange() {
	q.mu.Lock()
	onChange := q.onChange
	q.mu.Unlock()
	if onChange != nil {
		onChange()
	}
	q.emitEvent(EventState, "operations")
}

func displayDir(dir string) string {
	if dir == "" {
		return "(home)"
	}
	return dir
}
