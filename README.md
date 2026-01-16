# Claude Code Permissions Hook (Shell-Parsing Edition)

A PreToolUse hook for Claude Code that uses **proper shell parsing** instead of regex to validate commands. This allows for intelligent handling of compound commands like `git add -A && git commit -m "msg"`.

## Safety Notice

This tool can auto-approve operations, which can be risky. Use it only if you understand the security implications and are comfortable being accountable for the resulting tool actions. Start with narrow allowlists and expand cautiously. Think of it as giving your robot your credit card: handy, but still your responsibility when your account is empty.

## The Annoying Bits

Claude Code's built-in permissions and existing regex-based hooks struggle with:

1. **Repetitive permission requests**: You keep approving `timeout 30 dotnet run`, `timeout 45 dotnet run`, `timeout 50 dotnet run`... (yes, all three. every. single. time.)
2. **Compound commands**: How do you allow `git add && git commit` but not `git add && git push`?
3. **Shell injection risks**: Regex patterns are hard to make safe against `cmd; malicious`

## How This Tool Helps

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

## Quickstart (2 minutes)

```bash
# 1. Install
go install github.com/asbjornb/claude-hooks/claude-permissions-hook@latest

# 2. Initialize config (generates ~/.config/claude-permissions.toml)
claude-permissions-hook init

# 3. Open Claude Code, run /hooks, and add:
#    Matcher: Bash
#    Command: claude-permissions-hook run --config ~/.config/claude-permissions.toml
```

That's it. Your hook is now parsing shell commands instead of playing regex whack-a-mole.

The [default config](claude-permissions-hook/default-config.toml) allows safe git commands (no history manipulation). See [example.toml](example.toml) for a more complete configuration with dotnet, npm, and other stacks.

## Common Recipes

### Git Commit Flow (without push)

Allow the full commit dance, but block `git push` so you stay in control of the remote:

```toml
[[deny]]
tool = "Bash"
description = "Block push - I'll do this myself"
commands = ["git push"]

[[allow]]
tool = "Bash"
description = "Git commit workflow"
commands = ["git add", "git commit", "git status", "git diff", "git log"]
exclude_patterns = ["&", ";", "\\|", "`", "\\$\\("]
```

Now `git add -A && git commit -m "fix typo"` just works. Push stays in your hands.

### dotnet with timeout

Stop approving every timeout variation:

```toml
[[allow]]
tool = "Bash"
description = "dotnet with timeout wrapper"
commands = ["timeout dotnet", "dotnet build", "dotnet run", "dotnet test"]
exclude_patterns = ["&", ";", "\\|", "`", "\\$\\("]
```

The prefix `timeout dotnet` matches `timeout 30 dotnet run`, `timeout 120 dotnet test`, etc.

### Node.js Development

```toml
[[allow]]
tool = "Bash"
description = "Node.js tooling"
commands = ["npm install", "npm run", "npm test", "yarn build", "pnpm run"]
exclude_patterns = ["&", ";", "\\|", "`", "\\$\\("]
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

For compound commands (`&&`, `||`, `;`, `|`), **every** command is validated:

| Command | Result | Why |
|---------|--------|-----|
| `git add -A && git commit -m "x"` | ✅ ALLOW | Both commands on allow list |
| `git add -A && git push` | 🚫 DENY | git push on deny list → blocked entirely |
| `git add -A && curl example.com` | ⏸ PASSTHROUGH | curl not in any rule → user decides |

### 4. Deny Rules for Hard Blocks

Deny rules block commands entirely - Claude cannot proceed, and you'll have to do it yourself:

```toml
[[deny]]
tool = "Bash"
description = "Block push to remote"
commands = ["git push"]
```

With this rule, Claude will never push for you. You stay in control of what goes to the remote.

### 5. Session Allowlist Analysis

Import your existing Claude Code session allowlists to generate suggested patterns:

```bash
# Extract your session permissions (usually at .claude/settings.local.json)
claude-permissions-hook analyze --allowlist .claude/settings.local.json --format toml
```

## Installation

Requires Go 1.22+:

```bash
# Install directly
go install github.com/asbjornb/claude-hooks/claude-permissions-hook@latest

# Or build from source
git clone https://github.com/asbjornb/claude-hooks
cd claude-hooks/claude-permissions-hook
go build -o claude-permissions-hook .
```

Why Go? I wanted a small, fast-starting single binary with minimal runtime dependencies. Rust would work too, but Go keeps the tool simple and quick to iterate.

## Configuration

Create a TOML configuration file (see `example.toml`):

```toml
[audit]
audit_file = "/tmp/claude-permissions.json"
audit_level = "matched"  # off, matched, all

# Deny rules - checked first
[[deny]]
tool = "Bash"
description = "Block push to remote"
commands = ["git push"]

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

Run `/hooks` in Claude Code and add a PreToolUse hook with:
- **Matcher**: `Bash` (or `Bash|Read|Write|Edit` for file operations too)
- **Command**: `claude-permissions-hook run --config ~/.config/claude-permissions.toml`

Or manually add to `.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "claude-permissions-hook run --config ~/.config/claude-permissions.toml"
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

### `init` - Generate Config

```bash
claude-permissions-hook init  # Creates ~/.config/claude-permissions.toml
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
