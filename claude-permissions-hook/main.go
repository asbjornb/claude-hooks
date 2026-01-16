package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/user/claude-permissions-hook/config"
	"github.com/user/claude-permissions-hook/hook"
	"github.com/user/claude-permissions-hook/matcher"
	"github.com/user/claude-permissions-hook/parser"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "validate":
		validateCmd(os.Args[2:])
	case "analyze":
		analyzeCmd(os.Args[2:])
	case "parse":
		parseCmd(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`claude-permissions-hook - A PreToolUse hook for Claude Code

Commands:
  run       Run as a Claude Code hook (reads JSON from stdin)
  validate  Validate a configuration file
  analyze   Analyze a session allowlist and suggest patterns
  parse     Parse a shell command and show its structure

Usage:
  claude-permissions-hook run --config <config.toml>
  claude-permissions-hook validate --config <config.toml>
  claude-permissions-hook analyze --allowlist <permissions.json>
  claude-permissions-hook parse <command>

For more information, see the README.md`)
}

// runCmd executes the hook using the provided configuration
func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to TOML configuration file")
	fs.Parse(args)

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --config is required")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	input, err := hook.ReadInput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	m := matcher.New(cfg)
	var result matcher.MatchResult

	switch input.ToolName {
	case "Bash":
		cmd := input.GetBashCommand()
		if cmd == "" {
			hook.WritePassthrough()
			return
		}
		result = m.MatchBashCommand(cmd)

	case "Read", "Write", "Edit":
		path := input.GetFilePath()
		if path == "" {
			hook.WritePassthrough()
			return
		}
		result = m.MatchFilePath(input.ToolName, path)

	default:
		// Passthrough for other tools
		hook.WritePassthrough()
		return
	}

	// Write audit entry if enabled
	if cfg.Audit.AuditFile != "" {
		shouldAudit := false
		switch cfg.Audit.AuditLevel {
		case "all":
			shouldAudit = true
		case "matched":
			shouldAudit = result.Decision != matcher.DecisionPassthrough
		}

		if shouldAudit {
			entry := hook.AuditEntry{
				SessionID: input.SessionID,
				ToolName:  input.ToolName,
				ToolInput: input.ToolInput,
				Decision:  string(result.Decision),
				Reason:    result.Reason,
				RuleMatch: result.MatchedRule,
				Details:   result.Details,
			}
			hook.WriteAuditEntry(cfg.Audit.AuditFile, entry)
		}
	}

	// Output decision
	switch result.Decision {
	case matcher.DecisionAllow:
		reason := result.Reason
		if result.MatchedRule != "" {
			reason = result.MatchedRule + ": " + reason
		}
		hook.WriteAllow(reason)
	case matcher.DecisionDeny:
		reason := result.Reason
		if result.MatchedRule != "" {
			reason = result.MatchedRule + ": " + reason
		}
		hook.WriteDeny(reason)
	case matcher.DecisionPassthrough:
		hook.WritePassthrough()
	}
}

// validateCmd validates a configuration file
func validateCmd(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to TOML configuration file")
	fs.Parse(args)

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --config is required")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Configuration invalid: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Configuration valid")
	fmt.Printf("   Allow rules: %d\n", len(cfg.Allow))
	fmt.Printf("   Deny rules: %d\n", len(cfg.Deny))
	fmt.Printf("   Audit level: %s\n", cfg.Audit.AuditLevel)
	if cfg.Audit.AuditFile != "" {
		fmt.Printf("   Audit file: %s\n", cfg.Audit.AuditFile)
	}
}

// SessionPermissions represents the JSON format from Claude Code session allowlists
type SessionPermissions struct {
	Permissions struct {
		Allow []string `json:"allow"`
		Deny  []string `json:"deny"`
	} `json:"permissions"`
}

// CommandGroup represents a group of similar commands
type CommandGroup struct {
	Pattern  string
	Examples []string
	Count    int
}

// analyzeCmd analyzes a session allowlist and suggests patterns
func analyzeCmd(args []string) {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	allowlistPath := fs.String("allowlist", "", "Path to session permissions JSON file")
	outputFormat := fs.String("format", "toml", "Output format: toml or text")
	fs.Parse(args)

	if *allowlistPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --allowlist is required")
		os.Exit(1)
	}

	data, err := os.ReadFile(*allowlistPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading allowlist: %v\n", err)
		os.Exit(1)
	}

	var perms SessionPermissions
	if err := json.Unmarshal(data, &perms); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing allowlist: %v\n", err)
		os.Exit(1)
	}

	groups := analyzePermissions(perms.Permissions.Allow)

	if *outputFormat == "toml" {
		printTOMLSuggestions(groups)
	} else {
		printTextSuggestions(groups)
	}
}

// parseCmd parses a shell command and shows its structure
func parseCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: command required")
		os.Exit(1)
	}

	cmd := strings.Join(args, " ")
	stmt, err := parser.ParseShellCommand(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing command: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Command: %s\n", cmd)
	fmt.Printf("Parsed %d command(s):\n", len(stmt.Commands))

	for i, c := range stmt.Commands {
		fmt.Printf("\n  [%d] %s\n", i+1, c.Raw)
		fmt.Printf("      Name: %s\n", c.Name)
		fmt.Printf("      Args: %v\n", c.Args)
		fmt.Printf("      Signature: %s\n", parser.CommandSignature(c))
		if c.Operator != "" {
			fmt.Printf("      Next operator: %s\n", c.Operator)
		}
	}

	if stmt.HasPipe {
		fmt.Println("\n  ⚠️  Contains pipe")
	}
	if stmt.HasSubshell {
		fmt.Println("\n  ⚠️  Contains subshell")
	}
	if stmt.HasBackground {
		fmt.Println("\n  ⚠️  Contains background job")
	}
}

// analyzePermissions groups similar permissions and suggests patterns
func analyzePermissions(perms []string) []CommandGroup {
	// Parse Claude Code permission format: "Bash(command:*)" or "Bash(full command)"
	bashPattern := regexp.MustCompile(`^Bash\((.+?)(?::\*)?\)$`)

	commandSigs := make(map[string][]string)

	for _, perm := range perms {
		matches := bashPattern.FindStringSubmatch(perm)
		if matches == nil {
			continue
		}

		cmd := matches[1]

		// Parse the command to get its signature
		stmt, err := parser.ParseShellCommand(cmd)
		if err != nil {
			continue
		}

		for _, c := range stmt.Commands {
			sig := parser.CommandSignature(c)
			commandSigs[sig] = append(commandSigs[sig], cmd)
		}
	}

	// Convert to groups
	var groups []CommandGroup
	for sig, examples := range commandSigs {
		groups = append(groups, CommandGroup{
			Pattern:  sig,
			Examples: unique(examples),
			Count:    len(examples),
		})
	}

	// Sort by count descending
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Count > groups[j].Count
	})

	return groups
}

func unique(strs []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func printTOMLSuggestions(groups []CommandGroup) {
	fmt.Println("# Suggested configuration based on session allowlist")
	fmt.Println("# Review and customize before using")
	fmt.Println()

	// Group by command name for cleaner output
	byCommand := make(map[string][]CommandGroup)
	for _, g := range groups {
		parts := strings.Fields(g.Pattern)
		if len(parts) > 0 {
			byCommand[parts[0]] = append(byCommand[parts[0]], g)
		}
	}

	for cmd, cmdGroups := range byCommand {
		fmt.Printf("# %s commands (%d patterns)\n", cmd, len(cmdGroups))
		fmt.Println("[[allow]]")
		fmt.Println("tool = \"Bash\"")
		fmt.Printf("description = \"%s commands\"\n", cmd)

		var cmds []string
		for _, g := range cmdGroups {
			cmds = append(cmds, g.Pattern)
		}
		fmt.Printf("commands = %s\n", toTOMLArray(cmds))
		fmt.Println("exclude_patterns = [\"&|;|\\\\||`|\\\\$\\\\(\"]  # Block shell injection")
		fmt.Println()
	}
}

func printTextSuggestions(groups []CommandGroup) {
	fmt.Println("Suggested command patterns:")
	fmt.Println("===========================")
	fmt.Println()

	for _, g := range groups {
		fmt.Printf("Pattern: %s\n", g.Pattern)
		fmt.Printf("  Count: %d\n", g.Count)
		if len(g.Examples) <= 3 {
			fmt.Printf("  Examples: %v\n", g.Examples)
		} else {
			fmt.Printf("  Examples: %v ... (%d more)\n", g.Examples[:3], len(g.Examples)-3)
		}
		fmt.Println()
	}
}

func toTOMLArray(strs []string) string {
	var quoted []string
	for _, s := range strs {
		quoted = append(quoted, fmt.Sprintf(`"%s"`, s))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
