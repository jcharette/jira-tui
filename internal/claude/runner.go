package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Enabled bool
	Command string
	Timeout time.Duration
}

type Request struct {
	Config   Config
	Prompt   string
	Progress func(Event)
}

type Result struct {
	Text string
}

type Event struct {
	Kind string
	Text string
}

type Status struct {
	Enabled   bool
	Available bool
	Command   string
	Version   string
	Message   string
	Err       error
	CheckedAt time.Time
}

type LocalRunner struct {
	LookPath   func(string) (string, error)
	RunVersion func(context.Context, string) (string, error)
	RunPrompt  func(context.Context, string, []string) (string, error)
	Now        func() time.Time
}

func (r LocalRunner) Check(ctx context.Context, cfg Config) Status {
	status := Status{
		Enabled:   cfg.Enabled,
		CheckedAt: r.now(),
	}
	if !cfg.Enabled {
		status.Message = "Claude disabled"
		return status
	}

	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		found, err := r.lookPath("claude")
		if err != nil {
			status.Message = "Claude command not found"
			status.Err = err
			return status
		}
		command = found
	}
	status.Command = command

	checkCtx := ctx
	cancel := func() {}
	if cfg.Timeout > 0 {
		checkCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
	}
	defer cancel()

	version, err := r.runVersion(checkCtx, command)
	if err != nil {
		status.Message = "Claude version check failed"
		status.Err = err
		return status
	}
	status.Available = true
	status.Version = strings.TrimSpace(version)
	status.Message = "Claude ready"
	return status
}

func (r LocalRunner) lookPath(file string) (string, error) {
	if r.LookPath != nil {
		return r.LookPath(file)
	}
	return exec.LookPath(file)
}

func (r LocalRunner) Run(ctx context.Context, request Request) (Result, error) {
	if !request.Config.Enabled {
		return Result{}, errors.New("Claude disabled")
	}
	prompt := strings.TrimSpace(request.Prompt)
	if prompt == "" {
		return Result{}, errors.New("Claude prompt is required")
	}
	command, err := r.resolveCommand(request.Config.Command)
	if err != nil {
		return Result{}, err
	}
	runCtx := ctx
	cancel := func() {}
	if request.Config.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, request.Config.Timeout)
	}
	defer cancel()
	var output string
	if request.Progress != nil && r.RunPrompt == nil {
		output, err = r.runPromptStreaming(runCtx, command, prompt, request.Progress)
	} else {
		output, err = r.runPrompt(runCtx, command, []string{"-p", prompt})
	}
	if runCtx.Err() != nil {
		return Result{}, runCtx.Err()
	}
	if err != nil {
		return Result{}, err
	}
	text := strings.TrimSpace(output)
	if text == "" {
		return Result{}, errors.New("Claude returned empty output")
	}
	return Result{Text: text}, nil
}

func (r LocalRunner) runPromptStreaming(ctx context.Context, command string, prompt string, progress func(Event)) (string, error) {
	args := []string{"--verbose", "--output-format", "stream-json", "--include-partial-messages", "-p", prompt}
	cmd := exec.CommandContext(ctx, command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	var mu sync.Mutex
	var finalText strings.Builder
	var fallbackText strings.Builder
	var stderrText strings.Builder
	var progressMu sync.Mutex
	emitProgress := func(event Event) {
		progressMu.Lock()
		defer progressMu.Unlock()
		progress(event)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanClaudeStdout(stdout, emitProgress, func(text string, final bool) {
			mu.Lock()
			defer mu.Unlock()
			if final {
				finalText.WriteString(text)
				return
			}
			fallbackText.WriteString(text)
		})
	}()
	go func() {
		defer wg.Done()
		scanLines(stderr, func(line string) {
			if line == "" {
				return
			}
			emitProgress(Event{Kind: "stderr", Text: line})
			mu.Lock()
			defer mu.Unlock()
			stderrText.WriteString(line)
			stderrText.WriteString("\n")
		})
	}()
	err = cmd.Wait()
	wg.Wait()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	mu.Lock()
	defer mu.Unlock()
	if err != nil {
		if stderr := strings.TrimSpace(stderrText.String()); stderr != "" {
			return "", errors.Join(err, errors.New(stderr))
		}
		return "", err
	}
	if text := strings.TrimSpace(finalText.String()); text != "" {
		return text, nil
	}
	return strings.TrimSpace(fallbackText.String()), nil
}

func scanClaudeStdout(reader io.Reader, progress func(Event), appendText func(string, bool)) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		event, text, final := parseClaudeStreamLine(line)
		if event.Text != "" {
			progress(event)
		}
		if text != "" {
			appendText(text, final)
		}
	}
}

func scanLines(reader io.Reader, onLine func(string)) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		onLine(strings.TrimSpace(scanner.Text()))
	}
}

func parseClaudeStreamLine(line string) (Event, string, bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return Event{Kind: "stdout", Text: line}, line, false
	}
	kind := stringField(payload, "type")
	if kind == "" {
		kind = "stdout"
	}
	if kind == "user" {
		return Event{}, "", false
	}
	if kind == "assistant" {
		if text := firstText(payload); text != "" {
			return Event{Kind: "output", Text: text}, text, false
		}
		return Event{}, "", false
	}
	if kind == "stream_event" {
		if nested, ok := payload["event"].(map[string]any); ok {
			if text := firstText(nested); text != "" {
				return Event{Kind: "output", Text: text}, text, false
			}
			if nestedKind := stringField(nested, "type"); nestedKind != "" {
				return Event{}, "", false
			}
		}
	}
	if result := stringField(payload, "result"); result != "" {
		return Event{Kind: "result", Text: result}, result, true
	}
	if message := stringField(payload, "message"); message != "" {
		return Event{Kind: kind, Text: message}, "", false
	}
	if text := firstText(payload); text != "" {
		return Event{Kind: "output", Text: text}, text, false
	}
	return Event{Kind: kind, Text: truncateLine(line, 160)}, "", false
}

func readableStreamEvent(kind string) string {
	kind = strings.TrimSpace(strings.ReplaceAll(kind, "_", " "))
	if kind == "" {
		return ""
	}
	return kind
}

func stringField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func firstText(value any) string {
	switch value := value.(type) {
	case map[string]any:
		if text := stringField(value, "text"); text != "" {
			return text
		}
		for _, nested := range value {
			if text := firstText(nested); text != "" {
				return text
			}
		}
	case []any:
		for _, nested := range value {
			if text := firstText(nested); text != "" {
				return text
			}
		}
	}
	return ""
}

func truncateLine(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	if maxLen <= 1 {
		return value[:maxLen]
	}
	return value[:maxLen-1] + "..."
}

func (r LocalRunner) resolveCommand(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command != "" {
		return command, nil
	}
	found, err := r.lookPath("claude")
	if err != nil {
		return "", err
	}
	return found, nil
}

func (r LocalRunner) runPrompt(ctx context.Context, command string, args []string) (string, error) {
	if r.RunPrompt != nil {
		return r.RunPrompt(ctx, command, args)
	}
	output, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", errors.Join(err, errors.New(strings.TrimSpace(string(output))))
	}
	return string(output), nil
}

func (r LocalRunner) runVersion(ctx context.Context, command string) (string, error) {
	if r.RunVersion != nil {
		return r.RunVersion(ctx, command)
	}
	output, err := exec.CommandContext(ctx, command, "--version").CombinedOutput()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", errors.Join(err, errors.New(strings.TrimSpace(string(output))))
	}
	return string(output), nil
}

func (r LocalRunner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}
