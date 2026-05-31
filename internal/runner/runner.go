package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"syscall"

	"github.com/nickzhog/programmable-assistant/internal/config"
	"github.com/nickzhog/programmable-assistant/internal/sanitizer"
)

type RunOptions struct {
	WorkDir           string
	Alias             config.Alias
	AliasKey          string
	Mode              string
	Prompt            string
	OpenCodeSessionID string
	Fork              bool
}

type OutputChunk struct {
	Text       string
	Err        error
	ToolCall   *ToolCallInfo
	ToolResult *ToolResultInfo
}

type ToolCallInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

type ToolResultInfo struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
}

type runState struct {
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

type Runner struct {
	mu      sync.Mutex
	running map[string]*runState
}

type RunHandle struct {
	ch        <-chan OutputChunk
	sessionCh <-chan string
}

func (h *RunHandle) Output() <-chan OutputChunk {
	return h.ch
}

func (h *RunHandle) SessionID() <-chan string {
	return h.sessionCh
}

func New() *Runner {
	return &Runner{
		running: make(map[string]*runState),
	}
}

type opencodeEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID"`
	Part      struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"part"`
	ToolCall   *opencodeToolCall   `json:"tool_call,omitempty"`
	ToolResult *opencodeToolResult `json:"tool_result,omitempty"`
}

type opencodeToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

type opencodeToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
}

func (r *Runner) Run(ctx context.Context, sessionID string, opts RunOptions) (*RunHandle, error) {
	r.mu.Lock()
	if _, exists := r.running[sessionID]; exists {
		r.mu.Unlock()
		return nil, fmt.Errorf("session %s is already running", sessionID)
	}

	ctx, cancel := context.WithCancel(ctx)
	r.mu.Unlock()

	args := []string{
		"run",
		"--format", "json",
		"--log-level", "DEBUG",
	}

	if opts.OpenCodeSessionID != "" {
		args = append(args, "--session", opts.OpenCodeSessionID)
	}
	if opts.Fork {
		args = append(args, "--fork")
	}
	if opts.Alias.Thinking != "" {
		args = append(args, "--thinking")
	}
	if opts.Mode == "plan" {
		args = append(args, "--agent", "plan")
	}

	args = append(args,
		"--model", opts.Alias.Provider+"/"+opts.Alias.Model,
		opts.Prompt,
	)

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
		slog.Error("command start failed",
			"session", sessionID,
			"args", args,
			"error", err,
		)
		return nil, fmt.Errorf("start command: %w", err)
	}

	slog.Info("command started",
		"session", sessionID,
		"workdir", opts.WorkDir,
		"args", args,
	)

	rs := &runState{cancel: cancel, cmd: cmd}
	r.mu.Lock()
	r.running[sessionID] = rs
	r.mu.Unlock()

	ch := make(chan OutputChunk, 64)
	sessionCh := make(chan string, 1)
	handle := &RunHandle{ch: ch, sessionCh: sessionCh}

	go func() {
		defer close(ch)
		defer close(sessionCh)
		defer r.remove(sessionID)
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(2)

		go scanJSON(stdout, ch, sessionCh, &wg)
		go scanPipe(stderr, ch, &wg)

		wg.Wait()

		if err := cmd.Wait(); err != nil {
			if ctx.Err() == nil {
				slog.Error("command exited with error",
					"session", sessionID,
					"error", err,
				)
				ch <- OutputChunk{Err: fmt.Errorf("command: %v", err)}
			} else {
				slog.Info("command cancelled", "session", sessionID)
			}
		} else {
			slog.Info("command completed", "session", sessionID)
		}
	}()

	return handle, nil
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

func scanJSON(pipe io.Reader, ch chan<- OutputChunk, sessionCh chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event opencodeEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		if event.SessionID != "" && sessionCh != nil {
			sessionCh <- event.SessionID
			sessionCh = nil
		}

		if event.Part.Type == "text" && event.Part.Text != "" {
			ch <- OutputChunk{Text: event.Part.Text}
		}

		if event.ToolCall != nil {
			ch <- OutputChunk{
				ToolCall: &ToolCallInfo{
					ID:    event.ToolCall.ID,
					Name:  event.ToolCall.Name,
					Input: event.ToolCall.Input,
				},
			}
		}

		if event.ToolResult != nil {
			ch <- OutputChunk{
				ToolResult: &ToolResultInfo{
					ToolCallID: event.ToolResult.ToolCallID,
					Name:       event.ToolResult.Name,
					Content:    event.ToolResult.Content,
				},
			}
		}
	}
	if err := scanner.Err(); err != nil {
		ch <- OutputChunk{Err: fmt.Errorf("scanner: %w", err)}
	}
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
