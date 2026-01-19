// Package hook handles Claude Code hook input/output JSON format
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// HookInput represents the JSON input from Claude Code
type HookInput struct {
	SessionID      string                 `json:"session_id"`
	TranscriptPath string                 `json:"transcript_path"`
	Cwd            string                 `json:"cwd"`
	PermissionMode string                 `json:"permission_mode"`
	HookEventName  string                 `json:"hook_event_name"`
	ToolName       string                 `json:"tool_name"`
	ToolInput      map[string]interface{} `json:"tool_input"`
	ToolUseID      string                 `json:"tool_use_id"`
}

// HookOutput represents the JSON output to Claude Code
type HookOutput struct {
	// PermissionDecision controls whether to allow/deny the tool use
	// Values: "allow", "deny", "ask"
	PermissionDecision string `json:"permissionDecision,omitempty"`

	// PermissionDecisionReason is shown to Claude when denying
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`

	// Continue controls whether Claude should continue after the hook
	Continue *bool `json:"continue,omitempty"`

	// StopReason is shown when Continue is false
	StopReason string `json:"stopReason,omitempty"`
}

// AuditEntry represents a log entry for the audit file
type AuditEntry struct {
	Timestamp  string                 `json:"timestamp"`
	SessionID  string                 `json:"session_id"`
	ToolName   string                 `json:"tool_name"`
	ToolInput  map[string]interface{} `json:"tool_input"`
	Decision   string                 `json:"decision"`
	Reason     string                 `json:"reason"`
	RuleMatch  string                 `json:"rule_match,omitempty"`
	Details    string                 `json:"details,omitempty"`
}

// ReadInput reads and parses hook input from stdin
func ReadInput() (*HookInput, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdin: %w", err)
	}

	var input HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input JSON: %w", err)
	}

	return &input, nil
}

// WriteOutput writes the hook output to stdout
func WriteOutput(output *HookOutput) error {
	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// WriteAllow outputs an allow decision
func WriteAllow(reason string) error {
	return WriteOutput(&HookOutput{
		PermissionDecision:       "allow",
		PermissionDecisionReason: reason,
	})
}

// WriteDeny outputs a deny decision
func WriteDeny(reason string) error {
	return WriteOutput(&HookOutput{
		PermissionDecision:       "deny",
		PermissionDecisionReason: reason,
	})
}

// WritePassthrough outputs an "ask" decision (passthrough to Claude's normal permissions)
func WritePassthrough() {
	WriteOutput(&HookOutput{
		PermissionDecision: "ask",
	})
}

// GetBashCommand extracts the command from Bash tool input
func (h *HookInput) GetBashCommand() string {
	if cmd, ok := h.ToolInput["command"].(string); ok {
		return cmd
	}
	return ""
}

// GetFilePath extracts the file path from Read/Write/Edit tool input
func (h *HookInput) GetFilePath() string {
	if path, ok := h.ToolInput["file_path"].(string); ok {
		return path
	}
	return ""
}

// WriteAuditEntry writes an entry to the audit file
func WriteAuditEntry(auditFile string, entry AuditEntry) error {
	if auditFile == "" {
		return nil
	}

	entry.Timestamp = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	f, err := os.OpenFile(auditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}

	return nil
}
