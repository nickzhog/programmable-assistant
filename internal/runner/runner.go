package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"

	"github.com/nickzhog/programmable-assistant/internal/config"
	"github.com/nickzhog/programmable-assistant/internal/sanitizer"
)

type RunOptions struct {
	WorkDir  string
	Alias    config.Alias
	AliasKey string
	Mode     string
	Prompt   string
}

type OutputChunk struct {
	Text string
	Err  error
}

type runState struct {
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

type Runner struct {
	mu      sync.Mutex
	running map[string]*runState
}

func New() *Runner {
	return &Runner{
		running: make(map[string]*runState),
	}
}

func (r *Runner) Run(ctx context.Context, sessionID string, opts RunOptions) (<-chan OutputChunk, error) {
	r.mu.Lock()
	if _, exists := r.running[sessionID]; exists {
		r.mu.Unlock()
		return nil, fmt.Errorf("session %s is already running", sessionID)
	}

	ctx, cancel := context.WithCancel(ctx)
	r.mu.Unlock()

	args := []string{
		"--provider", opts.Alias.Provider,
		"--model", opts.Alias.Model,
		"--" + opts.Mode, opts.Prompt,
	}

	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Dir = opts.WorkDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start command: %w", err)
	}

	rs := &runState{cancel: cancel, cmd: cmd}
	r.mu.Lock()
	r.running[sessionID] = rs
	r.mu.Unlock()

	ch := make(chan OutputChunk, 64)

	go func() {
		defer close(ch)
		defer r.remove(sessionID)
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(2)

		go scanPipe(stdout, ch, &wg)
		go scanPipe(stderr, ch, &wg)

		wg.Wait()

		if err := cmd.Wait(); err != nil {
			if ctx.Err() == nil {
				ch <- OutputChunk{Err: fmt.Errorf("command: %v", err)}
			}
		}
	}()

	return ch, nil
}

func (r *Runner) Abort(sessionID string) {
	r.mu.Lock()
	rs, ok := r.running[sessionID]
	r.mu.Unlock()
	if !ok {
		return
	}

	if rs.cmd != nil && rs.cmd.Process != nil {
		pgid := -rs.cmd.Process.Pid
		syscall.Kill(pgid, syscall.SIGKILL)
	}
	rs.cancel()
}

func (r *Runner) IsRunning(sessionID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.running[sessionID]
	return ok
}

func (r *Runner) remove(sessionID string) {
	r.mu.Lock()
	delete(r.running, sessionID)
	r.mu.Unlock()
}

func scanPipe(pipe io.Reader, ch chan<- OutputChunk, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		text := sanitizer.StripANSI(scanner.Text())
		ch <- OutputChunk{Text: text}
	}
	if err := scanner.Err(); err != nil {
		ch <- OutputChunk{Err: fmt.Errorf("scanner: %w", err)}
	}
}
