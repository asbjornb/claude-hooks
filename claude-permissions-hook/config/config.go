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

	// For file operations - path matching
	PathPatterns        []string `toml:"path_patterns"`         // Regex patterns for file paths
	PathExcludePatterns []string `toml:"path_exclude_patterns"` // Patterns that should be denied

	// Description for logging
	Description string `toml:"description"`

	// Compiled patterns (internal use)
	compiledCommandPatterns []*regexp.Regexp
	compiledPathPatterns    []*regexp.Regexp
	compiledPathExclude     []*regexp.Regexp
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
		if err := cfg.Allow[i].Compile(); err != nil {
			return nil, fmt.Errorf("error compiling allow rule %d: %w", i, err)
		}
	}
	for i := range cfg.Deny {
		if err := cfg.Deny[i].Compile(); err != nil {
			return nil, fmt.Errorf("error compiling deny rule %d: %w", i, err)
		}
	}

	return &cfg, nil
}

// Compile compiles all regex patterns in the rule
func (r *Rule) Compile() error {
	// Compile command patterns
	for _, pattern := range r.CommandPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid command pattern %q: %w", pattern, err)
		}
		r.compiledCommandPatterns = append(r.compiledCommandPatterns, re)
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

	return nil
}

// GetCompiledCommandPatterns returns compiled command patterns
func (r *Rule) GetCompiledCommandPatterns() []*regexp.Regexp {
	return r.compiledCommandPatterns
}

// GetCompiledPathPatterns returns compiled path patterns
func (r *Rule) GetCompiledPathPatterns() []*regexp.Regexp {
	return r.compiledPathPatterns
}

// GetCompiledPathExclude returns compiled path exclude patterns
func (r *Rule) GetCompiledPathExclude() []*regexp.Regexp {
	return r.compiledPathExclude
}
