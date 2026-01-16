// Package config handles TOML configuration loading and validation
package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/BurntSushi/toml"
)

// Config is the root configuration structure
type Config struct {
	Audit  AuditConfig  `toml:"audit"`
	Allow  []Rule       `toml:"allow"`
	Deny   []Rule       `toml:"deny"`
	Import []string     `toml:"import"` // Import session allowlists
}

// AuditConfig controls logging behavior
type AuditConfig struct {
	AuditFile  string `toml:"audit_file"`
	AuditLevel string `toml:"audit_level"` // "off", "matched", "all"
}

// Rule defines an allow or deny rule
type Rule struct {
	// Tool is the Claude Code tool name (e.g., "Bash", "Read", "Write")
	Tool string `toml:"tool"`

	// For Bash commands - command matching
	Commands        []string `toml:"commands"`         // List of allowed command signatures (e.g., ["git add", "git commit"])
	CommandPatterns []string `toml:"command_patterns"` // Regex patterns for commands
	ExcludePatterns []string `toml:"exclude_patterns"` // Patterns that should be denied even if command matches

	// For file operations - path matching
	PathPatterns        []string `toml:"path_patterns"`         // Regex patterns for file paths
	PathExcludePatterns []string `toml:"path_exclude_patterns"` // Patterns that should be denied

	// Legacy regex support (for compatibility with korny's tool)
	CommandRegex        string `toml:"command_regex"`
	CommandExcludeRegex string `toml:"command_exclude_regex"`
	FilePathRegex       string `toml:"file_path_regex"`
	FilePathExcludeRegex string `toml:"file_path_exclude_regex"`

	// Description for logging
	Description string `toml:"description"`

	// Compiled patterns (internal use)
	compiledCommandPatterns []*regexp.Regexp
	compiledExcludePatterns []*regexp.Regexp
	compiledPathPatterns    []*regexp.Regexp
	compiledPathExclude     []*regexp.Regexp
	compiledCmdRegex        *regexp.Regexp
	compiledCmdExcludeRegex *regexp.Regexp
	compiledPathRegex       *regexp.Regexp
	compiledPathExcRegex    *regexp.Regexp
}

// Load reads and parses a TOML configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.Audit.AuditLevel == "" {
		cfg.Audit.AuditLevel = "matched"
	}

	// Compile patterns
	for i := range cfg.Allow {
		if err := cfg.Allow[i].compile(); err != nil {
			return nil, fmt.Errorf("error compiling allow rule %d: %w", i, err)
		}
	}
	for i := range cfg.Deny {
		if err := cfg.Deny[i].compile(); err != nil {
			return nil, fmt.Errorf("error compiling deny rule %d: %w", i, err)
		}
	}

	return &cfg, nil
}

// compile compiles all regex patterns in the rule
func (r *Rule) compile() error {
	var err error

	// Compile command patterns
	for _, pattern := range r.CommandPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid command pattern %q: %w", pattern, err)
		}
		r.compiledCommandPatterns = append(r.compiledCommandPatterns, re)
	}

	// Compile exclude patterns
	for _, pattern := range r.ExcludePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
		}
		r.compiledExcludePatterns = append(r.compiledExcludePatterns, re)
	}

	// Compile path patterns
	for _, pattern := range r.PathPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid path pattern %q: %w", pattern, err)
		}
		r.compiledPathPatterns = append(r.compiledPathPatterns, re)
	}

	// Compile path exclude patterns
	for _, pattern := range r.PathExcludePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid path exclude pattern %q: %w", pattern, err)
		}
		r.compiledPathExclude = append(r.compiledPathExclude, re)
	}

	// Legacy regex support
	if r.CommandRegex != "" {
		r.compiledCmdRegex, err = regexp.Compile(r.CommandRegex)
		if err != nil {
			return fmt.Errorf("invalid command_regex %q: %w", r.CommandRegex, err)
		}
	}
	if r.CommandExcludeRegex != "" {
		r.compiledCmdExcludeRegex, err = regexp.Compile(r.CommandExcludeRegex)
		if err != nil {
			return fmt.Errorf("invalid command_exclude_regex %q: %w", r.CommandExcludeRegex, err)
		}
	}
	if r.FilePathRegex != "" {
		r.compiledPathRegex, err = regexp.Compile(r.FilePathRegex)
		if err != nil {
			return fmt.Errorf("invalid file_path_regex %q: %w", r.FilePathRegex, err)
		}
	}
	if r.FilePathExcludeRegex != "" {
		r.compiledPathExcRegex, err = regexp.Compile(r.FilePathExcludeRegex)
		if err != nil {
			return fmt.Errorf("invalid file_path_exclude_regex %q: %w", r.FilePathExcludeRegex, err)
		}
	}

	return nil
}

// GetCompiledCommandPatterns returns compiled command patterns
func (r *Rule) GetCompiledCommandPatterns() []*regexp.Regexp {
	return r.compiledCommandPatterns
}

// GetCompiledExcludePatterns returns compiled exclude patterns
func (r *Rule) GetCompiledExcludePatterns() []*regexp.Regexp {
	return r.compiledExcludePatterns
}

// GetCompiledPathPatterns returns compiled path patterns
func (r *Rule) GetCompiledPathPatterns() []*regexp.Regexp {
	return r.compiledPathPatterns
}

// GetCompiledPathExclude returns compiled path exclude patterns
func (r *Rule) GetCompiledPathExclude() []*regexp.Regexp {
	return r.compiledPathExclude
}

// GetCompiledCmdRegex returns the compiled legacy command regex
func (r *Rule) GetCompiledCmdRegex() *regexp.Regexp {
	return r.compiledCmdRegex
}

// GetCompiledCmdExcludeRegex returns the compiled legacy command exclude regex
func (r *Rule) GetCompiledCmdExcludeRegex() *regexp.Regexp {
	return r.compiledCmdExcludeRegex
}

// GetCompiledPathRegex returns the compiled legacy path regex
func (r *Rule) GetCompiledPathRegex() *regexp.Regexp {
	return r.compiledPathRegex
}

// GetCompiledPathExcRegex returns the compiled legacy path exclude regex
func (r *Rule) GetCompiledPathExcRegex() *regexp.Regexp {
	return r.compiledPathExcRegex
}
