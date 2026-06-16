package claude

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCheckAutoDetectsClaudeCommand(t *testing.T) {
	runner := LocalRunner{
		LookPath: func(file string) (string, error) {
			if file != "claude" {
				t.Fatalf("LookPath file = %q", file)
			}
			return "/opt/homebrew/bin/claude", nil
		},
		RunVersion: func(ctx context.Context, command string) (string, error) {
			if command != "/opt/homebrew/bin/claude" {
				t.Fatalf("version command = %q", command)
			}
			return "claude 1.2.3\n", nil
		},
	}

	status := runner.Check(context.Background(), Config{Enabled: true, Timeout: time.Second})

	if !status.Available || status.Command != "/opt/homebrew/bin/claude" || status.Version != "claude 1.2.3" {
		t.Fatalf("status = %#v", status)
	}
}

func TestCheckUsesExplicitClaudeCommand(t *testing.T) {
	runner := LocalRunner{
		LookPath: func(file string) (string, error) {
			t.Fatalf("LookPath should not run for explicit command %q", file)
			return "", nil
		},
		RunVersion: func(ctx context.Context, command string) (string, error) {
			if command != "/usr/local/bin/claude" {
				t.Fatalf("version command = %q", command)
			}
			return "Claude Code 2.0.0", nil
		},
	}

	status := runner.Check(context.Background(), Config{Enabled: true, Command: "/usr/local/bin/claude", Timeout: time.Second})

	if !status.Available || status.Command != "/usr/local/bin/claude" || status.Version != "Claude Code 2.0.0" {
		t.Fatalf("status = %#v", status)
	}
}

func TestCheckDisabledDoesNotLookForClaude(t *testing.T) {
	runner := LocalRunner{
		LookPath: func(file string) (string, error) {
			t.Fatalf("LookPath should not run when disabled")
			return "", nil
		},
	}

	status := runner.Check(context.Background(), Config{})

	if status.Available || status.Enabled || status.Message != "Claude disabled" {
		t.Fatalf("status = %#v", status)
	}
}

func TestCheckReportsClaudeNotFound(t *testing.T) {
	runner := LocalRunner{
		LookPath: func(file string) (string, error) {
			return "", errors.New("not found")
		},
	}

	status := runner.Check(context.Background(), Config{Enabled: true, Timeout: time.Second})

	if status.Available || status.Command != "" || status.Err == nil {
		t.Fatalf("status = %#v", status)
	}
}

func TestRunExecutesPromptWithClaudePrintMode(t *testing.T) {
	runner := LocalRunner{
		RunPrompt: func(ctx context.Context, command string, args []string) (string, error) {
			if command != "/usr/local/bin/claude" {
				t.Fatalf("command = %q", command)
			}
			wantArgs := []string{"-p", "Plan ticket ABC-1"}
			if len(args) != len(wantArgs) {
				t.Fatalf("args = %#v", args)
			}
			for index := range args {
				if args[index] != wantArgs[index] {
					t.Fatalf("args = %#v", args)
				}
			}
			return "\nImplementation plan\n", nil
		},
	}

	result, err := runner.Run(context.Background(), Request{
		Config: Config{Enabled: true, Command: "/usr/local/bin/claude", Timeout: time.Second},
		Prompt: "Plan ticket ABC-1",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Text != "Implementation plan" {
		t.Fatalf("Text = %q", result.Text)
	}
}

func TestRunRejectsDisabledClaude(t *testing.T) {
	runner := LocalRunner{}

	_, err := runner.Run(context.Background(), Request{Prompt: "Plan ticket ABC-1"})
	if err == nil {
		t.Fatal("expected disabled Claude error")
	}
}

func TestRunReturnsDeadlineExceededAfterConfiguredTimeout(t *testing.T) {
	started := make(chan struct{})
	runner := LocalRunner{
		RunPrompt: func(ctx context.Context, command string, args []string) (string, error) {
			close(started)
			<-ctx.Done()
			return "", ctx.Err()
		},
	}

	start := time.Now()
	_, err := runner.Run(context.Background(), Request{
		Config: Config{Enabled: true, Command: "/usr/local/bin/claude", Timeout: 20 * time.Millisecond},
		Prompt: "Plan ticket ABC-1",
	})
	elapsed := time.Since(start)

	<-started
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v", err)
	}
	if elapsed < 20*time.Millisecond {
		t.Fatalf("deadline fired too early after %s", elapsed)
	}
}

func TestRunStreamsProgressAndStderrFromClaudeCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test helper is Unix-only")
	}
	dir := t.TempDir()
	command := filepath.Join(dir, "fake-claude")
	script := `#!/bin/sh
printf '{"type":"system","message":"starting"}\n'
printf '{"type":"assistant","message":{"content":[{"type":"text","text":"partial plan"}]}}\n'
printf 'auth warning\n' >&2
printf '{"type":"result","result":"final plan"}\n'
`
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
	var events []Event
	runner := LocalRunner{}

	result, err := runner.Run(context.Background(), Request{
		Config:   Config{Enabled: true, Command: command, Timeout: 5 * time.Second},
		Prompt:   "Plan ticket ABC-1",
		Progress: func(event Event) { events = append(events, event) },
	})

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Text != "final plan" {
		t.Fatalf("Text = %q", result.Text)
	}
	for _, want := range []Event{
		{Kind: "system", Text: "starting"},
		{Kind: "output", Text: "partial plan"},
		{Kind: "stderr", Text: "auth warning"},
	} {
		if !hasClaudeEvent(events, want) {
			t.Fatalf("missing event %#v in %#v", want, events)
		}
	}
}

func TestParseClaudeStreamEventExtractsDeltaText(t *testing.T) {
	line := `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"use ALB target groups"}}}`

	event, text, final := parseClaudeStreamLine(line)

	if final {
		t.Fatal("stream delta should not be final")
	}
	if event.Kind != "output" || event.Text != "use ALB target groups" || text != "use ALB target groups" {
		t.Fatalf("event=%#v text=%q", event, text)
	}
	if strings.Contains(event.Text, "stream_event") || strings.Contains(event.Text, "{") {
		t.Fatalf("raw JSON leaked into event text: %q", event.Text)
	}
}

func TestParseClaudeStreamEventSuppressesProtocolNoise(t *testing.T) {
	cases := []string{
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Create a read-only implementation plan"}]}}`,
		`{"type":"stream_event","event":{"type":"message_delta","usage":{"output_tokens":42}}}`,
		`{"type":"stream_event","event":{"type":"message_stop"}}`,
		`{"type":"assistant","message":{"model":"claude-opus","usage":{"output_tokens":42}}}`,
	}
	for _, line := range cases {
		event, text, final := parseClaudeStreamLine(line)
		if final {
			t.Fatalf("protocol event should not be final: %s", line)
		}
		if text != "" {
			t.Fatalf("protocol event appended text %q for %s", text, line)
		}
		if event.Text != "" {
			t.Fatalf("protocol event rendered text %#v for %s", event, line)
		}
	}
}

func hasClaudeEvent(events []Event, want Event) bool {
	for _, event := range events {
		if event.Kind == want.Kind && event.Text == want.Text {
			return true
		}
	}
	return false
}
