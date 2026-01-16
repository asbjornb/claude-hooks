# Claude Code Permissions Hook (Shell-Parsing Edition)

A PreToolUse hook for Claude Code that uses **proper shell parsing** instead of regex to validate commands. This allows for intelligent handling of compound commands like `git add -A && git commit -m "msg"`.

## Safety Notice

This tool can auto-approve operations, which can be risky. Use it only if you understand the security implications and are comfortable being accountable for the resulting tool actions. Start with narrow allowlists and expand cautiously. Think of it as giving your robot your credit card: handy, but still your responsibility when your account is empty.

## The Problem

Claude Code's built-in permissions and existing regex-based hooks struggle with:

1. **Repetitive permission requests**: You keep approving `timeout 30 dotnet run`, `timeout 45 dotnet run`, `timeout 50 dotnet run`...
2. **Compound commands**: How do you allow `git add && git commit` but not `git add && git push`?
3. **Shell injection risks**: Regex patterns are hard to make safe against `cmd; malicious`

## The Solution

This tool uses [mvdan.cc/sh](https://github.com/mvdan/sh) to parse shell commands into an AST, then validates each command individually:

```
git add -A && git commit -m "msg"
        ↓ (parsed)
[git add -A]  &&  [git commit -m "msg"]
        ↓ (validated separately)
    ✅ allowed      ✅ allowed
        ↓
    ✅ ALLOW (all commands pass)
```

## Key Features

### 1. Command Signature Matching

Instead of regex, match on semantic command signatures:

```toml
[[allow]]
tool = "Bash"
commands = ["git add", "git commit", "dotnet build"]
```

This matches:
- `git add -A`
- `git add .`
- `git commit -m "message"`
- `dotnet build --configuration Release`

### 2. Wrapper Command Understanding

The parser understands wrapper commands like `timeout`, `sudo`, `env`:

```toml
commands = ["timeout dotnet"]  # Prefix match for timeout 30/45/50/etc dotnet run/build/test
```

This matches:
- `timeout 30 dotnet run`
- `timeout 45 dotnet build`
- `timeout 120 dotnet test --no-build`

### 3. Compound Command Validation

For compound commands (`&&`, `||`, `;`, `|`), **every** command must be allowed:

| Command | git add allowed | git commit allowed | Result |
|---------|-----------------|-------------------|--------|
| `git add -A && git commit -m "x"` | ✅ | ✅ | ✅ ALLOW |
| `git add -A && git push` | ✅ | ❌ | ⏸ PASSTHROUGH |
| `git add -A && rm -rf /` | ✅ | 🚫 DENY | 🚫 DENY |

### 4. Session Allowlist Analysis

Import your existing Claude Code session allowlists to generate suggested patterns:

```bash
# Extract your session permissions (usually at .claude/settings.local.json)
claude-permissions-hook analyze --allowlist .claude/settings.local.json --format toml
```

## Installation

Requires Go 1.22+:

```bash
git clone https://github.com/user/claude-permissions-hook
cd claude-permissions-hook
go build -o claude-permissions-hook .

# Or install directly
go install github.com/user/claude-permissions-hook@latest
```

Why Go? I wanted a small, fast-starting single binary with minimal runtime dependencies, and I’m more comfortable shipping Go in this repo. Rust would be a good fit too, but Go keeps the tool simple and quick to iterate.

## Configuration

Create a TOML configuration file (see `example.toml`):

```toml
[audit]
audit_file = "/tmp/claude-permissions.json"
audit_level = "matched"  # off, matched, all

# Deny rules - checked first
[[deny]]
tool = "Bash"
description = "Block dangerous rm"
commands = ["rm -rf", "rm -fr"]

# Allow rules
[[allow]]
tool = "Bash"
description = "Git commands"
commands = ["git add", "git commit", "git status", "git diff"]
exclude_patterns = ["&", ";", "\\|", "`"]  # Block shell injection

[[allow]]
tool = "Bash"
description = "dotnet with timeout"
commands = ["timeout dotnet", "dotnet build", "dotnet run", "dotnet test"]
exclude_patterns = ["&", ";", "\\|", "`"]
```

## Claude Code Setup

Add to `.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/claude-permissions-hook run --config ~/.config/claude-permissions.toml"
          }
        ]
      },
      {
        "matcher": "Read|Write|Edit",
        "hooks": [
          {
            "type": "command", 
            "command": "/path/to/claude-permissions-hook run --config ~/.config/claude-permissions.toml"
          }
        ]
      }
    ]
  }
}
```

## Commands

### `run` - Execute as Hook

```bash
echo '<hook-json>' | claude-permissions-hook run --config config.toml
```

### `validate` - Check Configuration

```bash
claude-permissions-hook validate --config config.toml
```

### `analyze` - Import Session Allowlist

```bash
# Your session permissions (from Claude Code)
cat > perms.json << 'EOF'
{
  "permissions": {
    "allow": [
      "Bash(git add:*)",
      "Bash(timeout 30 dotnet run:*)",
      "Bash(timeout 45 dotnet run:*)",
      ...
    ]
  }
}
EOF

# Generate suggested TOML config
claude-permissions-hook analyze --allowlist perms.json --format toml
```

### `parse` - Debug Command Parsing

```bash
claude-permissions-hook parse "git add -A && git commit -m 'msg'"
```

Output:
```
Command: git add -A && git commit -m 'msg'
Parsed 2 command(s):

  [1] git add -A
      Name: git
      Args: [git add -A]
      Signature: git add
      Next operator: &&

  [2] git commit -m 'msg'
      Name: git
      Args: [git commit -m msg]
      Signature: git commit
```

## Configuration Reference

### Command Matching

```toml
[[allow]]
tool = "Bash"

# Exact command signatures (recommended)
commands = ["git add", "git commit"]

# Regex patterns (when needed)
command_patterns = ["^npm run \\w+$"]

# Exclude patterns - deny even if command matches
exclude_patterns = ["&", ";", "\\|", "`", "\\$\\("]

# Description for logging
description = "Git commands"
```

### Path Matching (Read/Write/Edit)

```toml
[[allow]]
tool = "Read"
path_patterns = ["^/home/user/projects/"]
path_exclude_patterns = ["\\.\\.", "node_modules"]
```

### Legacy Regex Support

For compatibility with kornysietsma's tool:

```toml
[[allow]]
tool = "Bash"
command_regex = "^git (add|commit|status)"
command_exclude_regex = "&|;|\\|"
```

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                    Claude Code                               │
│  Wants to run: git add -A && git commit -m "fix"            │
└─────────────────────┬───────────────────────────────────────┘
                      │ PreToolUse hook
                      ▼
┌─────────────────────────────────────────────────────────────┐
│              claude-permissions-hook                         │
│                                                              │
│  1. Parse shell command (mvdan.cc/sh)                       │
│     → [git add -A] && [git commit -m "fix"]                 │
│                                                              │
│  2. Check DENY rules on full command                        │
│     → No match                                              │
│                                                              │
│  3. For each parsed command:                                │
│     a. Extract signature: "git add", "git commit"           │
│     b. Check against ALLOW rules                            │
│     c. Verify no exclude patterns match                     │
│                                                              │
│  4. If ALL commands allowed → ALLOW                         │
│     If ANY command denied  → DENY                           │
│     Otherwise              → PASSTHROUGH                    │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│  Output: {"permissionDecision": "allow"}                    │
└─────────────────────────────────────────────────────────────┘
```

## Comparison with Regex Approach

| Feature | Regex (korny's tool) | Shell Parsing (this tool) |
|---------|---------------------|---------------------------|
| `timeout 30 dotnet` vs `timeout 45 dotnet` | Need separate patterns | Single pattern: `timeout dotnet` |
| `git add && git commit` | Complex regex | Validates each command |
| `cmd; malicious` | Easy to miss | Parsed as separate commands |
| `$(dangerous)` | Regex can miss | AST detects subshells |
| Performance | Fast (compiled regex) | Fast (Go, compiled) |

## Security Notes

1. **Subshell detection**: The parser flags `$(...)` and `` `...` `` constructs
2. **Exclude patterns**: Always use exclude patterns to block shell injection
3. **Deny first**: Deny rules are checked before allow rules
4. **Compound safety**: All commands in `&&`/`||`/`;` must be allowed

## License

MIT - See LICENSE file

## Credits

Inspired by [kornysietsma/claude-code-permissions-hook](https://github.com/kornysietsma/claude-code-permissions-hook).
Shell parsing powered by [mvdan.cc/sh](https://github.com/mvdan/sh).
