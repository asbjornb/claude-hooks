package parser

import (
	"testing"
)

func TestParseSimpleCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantArgs []string
		wantSig  string
	}{
		{
			name:     "simple ls",
			input:    "ls",
			wantName: "ls",
			wantArgs: []string{"ls"},
			wantSig:  "ls",
		},
		{
			name:     "ls with flags",
			input:    "ls -la",
			wantName: "ls",
			wantArgs: []string{"ls", "-la"},
			wantSig:  "ls",
		},
		{
			name:     "git add",
			input:    "git add -A",
			wantName: "git",
			wantArgs: []string{"git", "add", "-A"},
			wantSig:  "git add",
		},
		{
			name:     "git commit with message",
			input:    `git commit -m "test message"`,
			wantName: "git",
			wantArgs: []string{"git", "commit", "-m", "test message"},
			wantSig:  "git commit",
		},
		{
			name:     "dotnet build",
			input:    "dotnet build --configuration Release",
			wantName: "dotnet",
			wantArgs: []string{"dotnet", "build", "--configuration", "Release"},
			wantSig:  "dotnet build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseShellCommand(tt.input)
			if err != nil {
				t.Fatalf("ParseShellCommand() error = %v", err)
			}

			if len(stmt.Commands) != 1 {
				t.Fatalf("expected 1 command, got %d", len(stmt.Commands))
			}

			cmd := stmt.Commands[0]
			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}

			sig := CommandSignature(cmd)
			if sig != tt.wantSig {
				t.Errorf("Signature = %q, want %q", sig, tt.wantSig)
			}
		})
	}
}

func TestParseCompoundCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantSigs  []string
		wantOps   []string
	}{
		{
			name:      "git add && commit",
			input:     `git add -A && git commit -m "msg"`,
			wantCount: 2,
			wantSigs:  []string{"git add", "git commit"},
			wantOps:   []string{"&&", ""},
		},
		{
			name:      "three commands",
			input:     "ls && pwd && echo hello",
			wantCount: 3,
			wantSigs:  []string{"ls", "pwd", "echo"},
			wantOps:   []string{"&&", "&&", ""},
		},
		{
			name:      "pipe",
			input:     "cat file | grep pattern",
			wantCount: 2,
			wantSigs:  []string{"cat", "grep"},
			wantOps:   []string{"|", ""},
		},
		{
			name:      "or operator",
			input:     "test -f file || echo missing",
			wantCount: 2,
			wantSigs:  []string{"test", "echo"},
			wantOps:   []string{"||", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseShellCommand(tt.input)
			if err != nil {
				t.Fatalf("ParseShellCommand() error = %v", err)
			}

			if len(stmt.Commands) != tt.wantCount {
				t.Fatalf("command count = %d, want %d", len(stmt.Commands), tt.wantCount)
			}

			for i, cmd := range stmt.Commands {
				sig := CommandSignature(cmd)
				if i < len(tt.wantSigs) && sig != tt.wantSigs[i] {
					t.Errorf("command[%d] signature = %q, want %q", i, sig, tt.wantSigs[i])
				}
			}
		})
	}
}

func TestParseTimeoutWrapper(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSig string
	}{
		{
			name:    "timeout 30",
			input:   "timeout 30 dotnet run",
			wantSig: "timeout dotnet run",
		},
		{
			name:    "timeout 45",
			input:   "timeout 45 dotnet run",
			wantSig: "timeout dotnet run",
		},
		{
			name:    "timeout 120 with args",
			input:   "timeout 120 dotnet run --project src/App",
			wantSig: "timeout dotnet run",
		},
		{
			name:    "timeout with npm",
			input:   "timeout 60 npm run build",
			wantSig: "timeout npm run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseShellCommand(tt.input)
			if err != nil {
				t.Fatalf("ParseShellCommand() error = %v", err)
			}

			if len(stmt.Commands) != 1 {
				t.Fatalf("expected 1 command, got %d", len(stmt.Commands))
			}

			sig := CommandSignature(stmt.Commands[0])
			if sig != tt.wantSig {
				t.Errorf("signature = %q, want %q", sig, tt.wantSig)
			}
		})
	}
}

func TestParseWrapperCommands(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSig string
	}{
		{
			name:    "env assignment",
			input:   "env FOO=bar npm run build",
			wantSig: "env npm run",
		},
		{
			name:    "sudo with subcommand",
			input:   "sudo -u root git status",
			wantSig: "sudo git status",
		},
		{
			name:    "env without assignment",
			input:   "env npm test",
			wantSig: "env npm test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseShellCommand(tt.input)
			if err != nil {
				t.Fatalf("ParseShellCommand() error = %v", err)
			}

			if len(stmt.Commands) != 1 {
				t.Fatalf("expected 1 command, got %d", len(stmt.Commands))
			}

			sig := CommandSignature(stmt.Commands[0])
			if sig != tt.wantSig {
				t.Errorf("signature = %q, want %q", sig, tt.wantSig)
			}
		})
	}
}

func TestDetectDangerousConstructs(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantSubshell   bool
		wantPipe       bool
		wantBackground bool
	}{
		{
			name:         "subshell",
			input:        "echo $(whoami)",
			wantSubshell: true,
		},
		{
			name:     "pipe",
			input:    "cat file | grep x",
			wantPipe: true,
		},
		{
			name:           "background",
			input:          "sleep 10 &",
			wantBackground: true,
		},
		{
			name:  "safe command",
			input: "git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseShellCommand(tt.input)
			if err != nil {
				t.Fatalf("ParseShellCommand() error = %v", err)
			}

			if stmt.HasSubshell != tt.wantSubshell {
				t.Errorf("HasSubshell = %v, want %v", stmt.HasSubshell, tt.wantSubshell)
			}
			if stmt.HasPipe != tt.wantPipe {
				t.Errorf("HasPipe = %v, want %v", stmt.HasPipe, tt.wantPipe)
			}
			if stmt.HasBackground != tt.wantBackground {
				t.Errorf("HasBackground = %v, want %v", stmt.HasBackground, tt.wantBackground)
			}
		})
	}
}

func TestPathBasename(t *testing.T) {
	tests := []struct {
		input   string
		wantSig string
	}{
		{
			input:   "/usr/bin/git status",
			wantSig: "git status",
		},
		{
			input:   "/home/linuxbrew/.linuxbrew/bin/glab ci list",
			wantSig: "glab ci",
		},
		{
			input:   "~/.dotnet/tools/dotnet-ef migrations add",
			wantSig: "dotnet-ef migrations",
		},
		{
			input:   "git -C /mnt/c/code/net/service-monitor-2 status",
			wantSig: "git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			stmt, err := ParseShellCommand(tt.input)
			if err != nil {
				t.Fatalf("ParseShellCommand() error = %v", err)
			}

			if len(stmt.Commands) != 1 {
				t.Fatalf("expected 1 command, got %d", len(stmt.Commands))
			}

			sig := CommandSignature(stmt.Commands[0])
			if sig != tt.wantSig {
				t.Errorf("signature = %q, want %q", sig, tt.wantSig)
			}
		})
	}
}
