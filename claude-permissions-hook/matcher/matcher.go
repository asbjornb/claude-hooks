// Package matcher provides command matching against configuration rules
package matcher

import (
	"strings"

	"github.com/user/claude-permissions-hook/config"
	"github.com/user/claude-permissions-hook/parser"
)

// Decision represents the result of matching a command against rules
type Decision string

const (
	DecisionAllow       Decision = "allow"
	DecisionDeny        Decision = "deny"
	DecisionPassthrough Decision = "passthrough" // No rule matched, use default permissions
)

// MatchResult contains the result of matching and additional context
type MatchResult struct {
	Decision    Decision
	Reason      string
	MatchedRule string // Description of the rule that matched
	Details     string // Additional details about what matched/didn't match
}

// Matcher holds compiled configuration and provides matching methods
type Matcher struct {
	cfg *config.Config
}

// New creates a new Matcher with the given configuration
func New(cfg *config.Config) *Matcher {
	return &Matcher{cfg: cfg}
}

// MatchBashCommand checks a bash command against all rules
// For compound commands (cmd1 && cmd2), ALL commands must be allowed for the result to be allow
func (m *Matcher) MatchBashCommand(command string) MatchResult {
	// Parse the shell command
	stmt, err := parser.ParseShellCommand(command)
	if err != nil {
		return MatchResult{
			Decision: DecisionPassthrough,
			Reason:   "Failed to parse command",
			Details:  err.Error(),
		}
	}

	// Check for dangerous constructs first
	if stmt.HasSubshell {
		// Check if any deny rule explicitly blocks subshells
		for _, rule := range m.cfg.Deny {
			if rule.Tool != "Bash" {
				continue
			}
			for _, excl := range rule.GetCompiledExcludePatterns() {
				if excl.MatchString(command) {
					return MatchResult{
						Decision:    DecisionDeny,
						Reason:      "Command contains denied pattern",
						MatchedRule: rule.Description,
						Details:     "Subshell detected and matched exclude pattern",
					}
				}
			}
		}
	}

	// First, check deny rules on the full command and each subcommand
	for _, rule := range m.cfg.Deny {
		if rule.Tool != "Bash" {
			continue
		}
		if match := m.matchBashRule(rule, command, stmt); match {
			return MatchResult{
				Decision:    DecisionDeny,
				Reason:      "Command matched deny rule",
				MatchedRule: rule.Description,
			}
		}
	}

	// For compound commands, each individual command must be allowed
	if len(stmt.Commands) > 1 {
		for _, cmd := range stmt.Commands {
			result := m.checkSingleCommand(cmd)
			if result.Decision != DecisionAllow {
				return MatchResult{
					Decision: DecisionPassthrough,
					Reason:   "Not all commands in compound statement are allowed",
					Details:  "Command not allowed: " + cmd.Raw,
				}
			}
		}
		// All commands allowed
		return MatchResult{
			Decision: DecisionAllow,
			Reason:   "All commands in compound statement are allowed",
		}
	}

	// Single command - check allow rules
	if len(stmt.Commands) == 1 {
		return m.checkSingleCommand(stmt.Commands[0])
	}

	return MatchResult{
		Decision: DecisionPassthrough,
		Reason:   "No commands parsed",
	}
}

// checkSingleCommand checks a single parsed command against allow rules
func (m *Matcher) checkSingleCommand(cmd parser.ParsedCommand) MatchResult {
	sig := parser.CommandSignature(cmd)

	for _, rule := range m.cfg.Allow {
		if rule.Tool != "Bash" {
			continue
		}

		// Check explicit command list first (most specific)
		for _, allowedCmd := range rule.Commands {
			if matchCommandSignature(allowedCmd, sig, cmd) {
				// Check exclude patterns
				if m.matchesExcludePatterns(rule, cmd.Raw) {
					continue // Skip this rule, try next
				}
				return MatchResult{
					Decision:    DecisionAllow,
					Reason:      "Command matches allowed signature",
					MatchedRule: rule.Description,
					Details:     "Matched: " + allowedCmd,
				}
			}
		}

		// Check regex patterns
		for _, re := range rule.GetCompiledCommandPatterns() {
			if re.MatchString(cmd.Raw) {
				// Check exclude patterns
				if m.matchesExcludePatterns(rule, cmd.Raw) {
					continue
				}
				return MatchResult{
					Decision:    DecisionAllow,
					Reason:      "Command matches allowed pattern",
					MatchedRule: rule.Description,
				}
			}
		}

		// Legacy regex support
		if re := rule.GetCompiledCmdRegex(); re != nil && re.MatchString(cmd.Raw) {
			if excl := rule.GetCompiledCmdExcludeRegex(); excl != nil && excl.MatchString(cmd.Raw) {
				continue
			}
			return MatchResult{
				Decision:    DecisionAllow,
				Reason:      "Command matches allowed regex",
				MatchedRule: rule.Description,
			}
		}
	}

	return MatchResult{
		Decision: DecisionPassthrough,
		Reason:   "No allow rule matched",
		Details:  "Command signature: " + sig,
	}
}

// matchCommandSignature checks if a command matches an allowed signature
func matchCommandSignature(pattern, sig string, cmd parser.ParsedCommand) bool {
	// Exact signature match
	if pattern == sig {
		return true
	}

	// Pattern with wildcard (e.g., "git *" matches any git command)
	if strings.HasSuffix(pattern, " *") {
		prefix := strings.TrimSuffix(pattern, " *")
		if strings.HasPrefix(sig, prefix) {
			return true
		}
		// Also check command name directly
		cmdName := parser.GetCommandName(cmd)
		if cmdName == prefix {
			return true
		}
	}

	// Prefix match for multi-word patterns (e.g., "timeout dotnet" matches "timeout dotnet run")
	if strings.Contains(pattern, " ") {
		if strings.HasPrefix(sig, pattern+" ") {
			return true
		}
	}

	// Just command name (e.g., "ls" matches "ls -la")
	if !strings.Contains(pattern, " ") {
		cmdName := parser.GetCommandName(cmd)
		if pattern == cmdName {
			return true
		}
	}

	return false
}

// matchesExcludePatterns checks if command matches any exclude patterns
func (m *Matcher) matchesExcludePatterns(rule config.Rule, command string) bool {
	for _, re := range rule.GetCompiledExcludePatterns() {
		if re.MatchString(command) {
			return true
		}
	}
	return false
}

// matchBashRule checks if a command matches a deny rule
func (m *Matcher) matchBashRule(rule config.Rule, fullCmd string, stmt *parser.ShellStatement) bool {
	// Check regex patterns against full command
	for _, re := range rule.GetCompiledCommandPatterns() {
		if re.MatchString(fullCmd) {
			return true
		}
	}

	// Check legacy regex
	if re := rule.GetCompiledCmdRegex(); re != nil && re.MatchString(fullCmd) {
		// If there's an exclude regex, check it
		if excl := rule.GetCompiledCmdExcludeRegex(); excl != nil {
			return !excl.MatchString(fullCmd)
		}
		return true
	}

	// Check command signatures against deny list
	for _, cmd := range stmt.Commands {
		sig := parser.CommandSignature(cmd)
		for _, deniedCmd := range rule.Commands {
			if matchCommandSignature(deniedCmd, sig, cmd) {
				return true
			}
		}
	}

	return false
}

// MatchFilePath checks a file path against rules for Read/Write/Edit operations
func (m *Matcher) MatchFilePath(toolName, filePath string) MatchResult {
	// Check deny rules first
	for _, rule := range m.cfg.Deny {
		if rule.Tool != toolName {
			continue
		}

		// Check path patterns
		for _, re := range rule.GetCompiledPathPatterns() {
			if re.MatchString(filePath) {
				return MatchResult{
					Decision:    DecisionDeny,
					Reason:      "Path matched deny rule",
					MatchedRule: rule.Description,
				}
			}
		}

		// Legacy regex
		if re := rule.GetCompiledPathRegex(); re != nil && re.MatchString(filePath) {
			if excl := rule.GetCompiledPathExcRegex(); excl != nil && !excl.MatchString(filePath) {
				return MatchResult{
					Decision:    DecisionDeny,
					Reason:      "Path matched deny regex",
					MatchedRule: rule.Description,
				}
			} else if excl == nil {
				return MatchResult{
					Decision:    DecisionDeny,
					Reason:      "Path matched deny regex",
					MatchedRule: rule.Description,
				}
			}
		}
	}

	// Check allow rules
	for _, rule := range m.cfg.Allow {
		if rule.Tool != toolName {
			continue
		}

		// Check path patterns
		for _, re := range rule.GetCompiledPathPatterns() {
			if re.MatchString(filePath) {
				// Check exclude patterns
				excluded := false
				for _, excl := range rule.GetCompiledPathExclude() {
					if excl.MatchString(filePath) {
						excluded = true
						break
					}
				}
				if !excluded {
					return MatchResult{
						Decision:    DecisionAllow,
						Reason:      "Path matched allow pattern",
						MatchedRule: rule.Description,
					}
				}
			}
		}

		// Legacy regex
		if re := rule.GetCompiledPathRegex(); re != nil && re.MatchString(filePath) {
			if excl := rule.GetCompiledPathExcRegex(); excl != nil && excl.MatchString(filePath) {
				continue
			}
			return MatchResult{
				Decision:    DecisionAllow,
				Reason:      "Path matched allow regex",
				MatchedRule: rule.Description,
			}
		}
	}

	return MatchResult{
		Decision: DecisionPassthrough,
		Reason:   "No rule matched for path",
	}
}
