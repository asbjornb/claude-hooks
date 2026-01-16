// Package parser provides shell command parsing using mvdan.cc/sh
package parser

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// ParsedCommand represents a single command extracted from a shell statement
type ParsedCommand struct {
	// Name is the command name (e.g., "git", "npm", "dotnet")
	Name string
	// Args is the full list of arguments including the command name
	Args []string
	// Raw is the original string representation of this command
	Raw string
	// Operator is the operator that connects this command to the next (&&, ||, ;, |, or "")
	Operator string
}

// ShellStatement represents a parsed shell statement that may contain multiple commands
type ShellStatement struct {
	// Commands is the list of individual commands in the statement
	Commands []ParsedCommand
	// Raw is the original shell statement
	Raw string
	// HasPipe indicates if any commands are connected via pipe
	HasPipe bool
	// HasBackground indicates if any command runs in background (&)
	HasBackground bool
	// HasSubshell indicates if statement contains subshell $(...)
	HasSubshell bool
}

// ParseShellCommand parses a shell command string and extracts all individual commands
func ParseShellCommand(command string) (*ShellStatement, error) {
	parser := syntax.NewParser()
	reader := strings.NewReader(command)

	file, err := parser.Parse(reader, "")
	if err != nil {
		return nil, err
	}

	stmt := &ShellStatement{
		Raw:      command,
		Commands: make([]ParsedCommand, 0),
	}

	// Walk the AST to extract commands
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.CallExpr:
			cmd := extractCommand(n)
			if cmd.Name != "" {
				stmt.Commands = append(stmt.Commands, cmd)
			}
		case *syntax.BinaryCmd:
			// Track operators
			switch n.Op {
			case syntax.Pipe, syntax.PipeAll:
				stmt.HasPipe = true
			}
		case *syntax.Stmt:
			if n.Background {
				stmt.HasBackground = true
			}
		case *syntax.CmdSubst:
			stmt.HasSubshell = true
		}
		return true
	})

	// Second pass to extract operators between commands
	stmt.Commands = extractWithOperators(file, stmt.Commands)

	return stmt, nil
}

// extractCommand extracts command info from a CallExpr node
func extractCommand(call *syntax.CallExpr) ParsedCommand {
	cmd := ParsedCommand{
		Args: make([]string, 0),
	}

	// Extract all arguments (first is the command name)
	for _, word := range call.Args {
		arg := wordToString(word)
		cmd.Args = append(cmd.Args, arg)
	}

	if len(cmd.Args) > 0 {
		cmd.Name = cmd.Args[0]
		cmd.Raw = strings.Join(cmd.Args, " ")
	}

	return cmd
}

// wordToString converts a syntax.Word to a string
func wordToString(word *syntax.Word) string {
	var parts []string
	for _, part := range word.Parts {
		parts = append(parts, partToString(part))
	}
	return strings.Join(parts, "")
}

// partToString converts a WordPart to string
func partToString(part syntax.WordPart) string {
	switch p := part.(type) {
	case *syntax.Lit:
		return p.Value
	case *syntax.SglQuoted:
		return p.Value
	case *syntax.DblQuoted:
		var parts []string
		for _, sub := range p.Parts {
			parts = append(parts, partToString(sub))
		}
		return strings.Join(parts, "")
	case *syntax.ParamExp:
		// Return parameter expansion as placeholder
		return "${" + p.Param.Value + "}"
	case *syntax.CmdSubst:
		// Return command substitution indicator
		return "$(...)"
	default:
		return ""
	}
}

// extractWithOperators does a second pass to capture operators between commands
func extractWithOperators(file *syntax.File, commands []ParsedCommand) []ParsedCommand {
	if len(commands) == 0 {
		return commands
	}

	result := make([]ParsedCommand, len(commands))
	copy(result, commands)

	idx := 0
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.BinaryCmd:
			if idx < len(result) {
				switch n.Op {
				case syntax.AndStmt:
					result[idx].Operator = "&&"
				case syntax.OrStmt:
					result[idx].Operator = "||"
				case syntax.Pipe:
					result[idx].Operator = "|"
				case syntax.PipeAll:
					result[idx].Operator = "|&"
				}
				idx++
			}
		case *syntax.Stmt:
			// Handle semicolon-separated statements
			if idx > 0 && idx < len(result) && result[idx-1].Operator == "" {
				result[idx-1].Operator = ";"
			}
		}
		return true
	})

	return result
}

// GetCommandName returns the base command name (handles paths like /usr/bin/git -> git)
func GetCommandName(cmd ParsedCommand) string {
	name := cmd.Name
	// Extract basename if it's a path
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		name = parts[len(parts)-1]
	}
	return name
}

var valueFlagsByCommand = map[string]map[string]bool{
	"git": {
		"-C":          true,
		"--git-dir":   true,
		"--work-tree": true,
		"-c":          true,
	},
	"dotnet": {
		"--project": true,
		"-p":        true,
	},
	"glab": {
		"-R":     true,
		"--repo": true,
	},
	"gh": {
		"-R":     true,
		"--repo": true,
	},
	"timeout": {
		"-k":           true,
		"-s":           true,
		"--signal":     true,
		"--kill-after": true,
	},
	"sudo": {
		"-u": true,
		"-g": true,
		"-h": true,
		"-p": true,
		"-U": true,
	},
	"env": {
		"-u": true,
		"-C": true,
	},
}

func flagTakesValue(cmdName, flag string) bool {
	if flags, ok := valueFlagsByCommand[cmdName]; ok {
		return flags[flag]
	}
	return false
}

var subcommandCommands = map[string]bool{
	"git":       true,
	"dotnet":    true,
	"glab":      true,
	"gh":        true,
	"npm":       true,
	"pnpm":      true,
	"yarn":      true,
	"cargo":     true,
	"kubectl":   true,
	"terraform": true,
	"docker":    true,
	"az":        true,
	"dotnet-ef": true,
}

func isSubcommandCommand(cmdName string) bool {
	return subcommandCommands[cmdName]
}

func isEnvAssignment(arg string) bool {
	return strings.Contains(arg, "=") && !strings.HasPrefix(arg, "=")
}

// GetSubcommand returns the subcommand for tools like git, dotnet, glab
// For "git commit -m msg", returns "commit"
// For "dotnet build", returns "build"
func GetSubcommand(cmd ParsedCommand) string {
	if len(cmd.Args) > 1 {
		cmdName := GetCommandName(cmd)
		args := cmd.Args[1:]
		for i := 0; i < len(args); i++ {
			arg := args[i]
			if strings.HasPrefix(arg, "-") {
				if flagTakesValue(cmdName, arg) && i+1 < len(args) {
					i++
				}
				continue
			}
			return arg
		}
	}
	return ""
}

// CommandSignature returns a canonical representation of the command for matching
// e.g., "git add" for "git add -A .", "timeout dotnet run" for "timeout 30 dotnet run"
func CommandSignature(cmd ParsedCommand) string {
	name := GetCommandName(cmd)

	// Special handling for wrapper commands like timeout, env, sudo
	wrappers := map[string]bool{
		"timeout": true,
		"env":     true,
		"sudo":    true,
		"nice":    true,
		"nohup":   true,
		"time":    true,
	}

	if wrappers[name] {
		args := cmd.Args[1:]
		actualIdx := -1
		for i := 0; i < len(args); i++ {
			arg := args[i]
			if strings.HasPrefix(arg, "-") {
				if flagTakesValue(name, arg) && i+1 < len(args) {
					i++
				}
				continue
			}
			if name == "timeout" && isNumeric(arg) {
				continue
			}
			if name == "env" && isEnvAssignment(arg) {
				continue
			}
			actualIdx = i
			break
		}
		if actualIdx >= 0 && actualIdx < len(args) {
			actualArgs := args[actualIdx:]
			actualCmd := ParsedCommand{
				Name: actualArgs[0],
				Args: actualArgs,
				Raw:  strings.Join(actualArgs, " "),
			}
			actualName := GetCommandName(actualCmd)
			if isSubcommandCommand(actualName) {
				subCmd := GetSubcommand(actualCmd)
				if subCmd != "" && !strings.HasPrefix(subCmd, "-") && !strings.HasPrefix(subCmd, "/") {
					return name + " " + actualName + " " + subCmd
				}
			}
			return name + " " + actualName
		}
	}

	// For normal commands, include subcommand if present
	if isSubcommandCommand(name) {
		subCmd := GetSubcommand(cmd)
		if subCmd != "" && !strings.HasPrefix(subCmd, "-") && !strings.HasPrefix(subCmd, "/") {
			return name + " " + subCmd
		}
	}

	return name
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
