package matcher

import (
	"testing"

	"github.com/asbjornb/claude-hooks/claude-permissions-hook/config"
)

func TestTimeoutDotnetPattern(t *testing.T) {
	// This is the key use case: one pattern should match all timeout variations
	cfg := &config.Config{
		Allow: []config.Rule{
			{
				Tool:        "Bash",
				Commands:    []string{"timeout dotnet"},
				Description: "dotnet with timeout",
			},
		},
	}

	m := New(cfg)

	tests := []struct {
		command string
		want    Decision
	}{
		{"timeout 30 dotnet run", DecisionAllow},
		{"timeout 45 dotnet run", DecisionAllow},
		{"timeout 50 dotnet run", DecisionAllow},
		{"timeout 35 dotnet run", DecisionAllow},
		{"timeout 55 dotnet run", DecisionAllow},
		{"timeout 120 dotnet test", DecisionAllow},
		{"timeout 60 dotnet build", DecisionAllow},
		{"timeout 30 dotnet run --project src/App", DecisionAllow},
		// These should NOT match
		{"dotnet run", DecisionPassthrough},         // no timeout wrapper
		{"timeout 30 npm run build", DecisionPassthrough}, // different command
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := m.MatchBashCommand(tt.command)
			if result.Decision != tt.want {
				t.Errorf("MatchBashCommand(%q) = %v, want %v (reason: %s)",
					tt.command, result.Decision, tt.want, result.Reason)
			}
		})
	}
}

func TestGitCommitFlow(t *testing.T) {
	cfg := &config.Config{
		Deny: []config.Rule{
			{
				Tool:        "Bash",
				Commands:    []string{"git push"},
				Description: "Block git push",
			},
		},
		Allow: []config.Rule{
			{
				Tool:        "Bash",
				Commands:    []string{"git add", "git commit", "git status", "git diff"},
				Description: "Git commands",
			},
		},
	}

	m := New(cfg)

	tests := []struct {
		command string
		want    Decision
	}{
		// Single commands
		{"git add -A", DecisionAllow},
		{"git commit -m 'test'", DecisionAllow},
		{"git status", DecisionAllow},
		{"git diff", DecisionAllow},
		{"git push", DecisionDeny},
		{"git push origin main", DecisionDeny},

		// Compound commands
		{"git add -A && git commit -m 'test'", DecisionAllow},
		{"git add -A && git push", DecisionDeny},  // push is denied
		{"git add -A && git rebase", DecisionPassthrough}, // rebase not in allow list
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := m.MatchBashCommand(tt.command)
			if result.Decision != tt.want {
				t.Errorf("MatchBashCommand(%q) = %v, want %v (reason: %s)",
					tt.command, result.Decision, tt.want, result.Reason)
			}
		})
	}
}

func TestDenyTakesPrecedence(t *testing.T) {
	cfg := &config.Config{
		Deny: []config.Rule{
			{
				Tool:        "Bash",
				Commands:    []string{"git push"},
				Description: "Block push",
			},
		},
		Allow: []config.Rule{
			{
				Tool:        "Bash",
				Commands:    []string{"git"},  // Would match all git commands
				Description: "All git",
			},
		},
	}

	m := New(cfg)

	// Even though "git" would allow git push, deny should take precedence
	result := m.MatchBashCommand("git push origin main")
	if result.Decision != DecisionDeny {
		t.Errorf("Expected DENY for 'git push origin main', got %v", result.Decision)
	}

	// But other git commands should be allowed
	result = m.MatchBashCommand("git status")
	if result.Decision != DecisionAllow {
		t.Errorf("Expected ALLOW for 'git status', got %v", result.Decision)
	}
}

func TestPipeComposition(t *testing.T) {
	cfg := &config.Config{
		Deny: []config.Rule{
			{
				Tool:        "Bash",
				Commands:    []string{"grep"},
				Description: "Block grep",
			},
		},
		Allow: []config.Rule{
			{
				Tool:        "Bash",
				Commands:    []string{"cat", "grep"},
				Description: "Text tools",
			},
		},
	}

	m := New(cfg)

	tests := []struct {
		command string
		want    Decision
	}{
		{"cat file | grep pattern", DecisionDeny},
		{"cat file | wc", DecisionPassthrough},
		{"cat file | cat", DecisionAllow},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := m.MatchBashCommand(tt.command)
			if result.Decision != tt.want {
				t.Errorf("MatchBashCommand(%q) = %v, want %v (reason: %s)",
					tt.command, result.Decision, tt.want, result.Reason)
			}
		})
	}
}

func TestWrapperAllow(t *testing.T) {
	cfg := &config.Config{
		Allow: []config.Rule{
			{
				Tool:        "Bash",
				Commands:    []string{"env npm run", "sudo git status"},
				Description: "Wrapper commands",
			},
		},
	}

	m := New(cfg)

	tests := []struct {
		command string
		want    Decision
	}{
		{"env FOO=bar npm run build", DecisionAllow},
		{"sudo -u root git status", DecisionAllow},
		{"sudo git commit -m 'x'", DecisionPassthrough},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := m.MatchBashCommand(tt.command)
			if result.Decision != tt.want {
				t.Errorf("MatchBashCommand(%q) = %v, want %v (reason: %s)",
					tt.command, result.Decision, tt.want, result.Reason)
			}
		})
	}
}
