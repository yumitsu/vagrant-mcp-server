package utils

import (
	"fmt"
	"strings"
)

// OutputParser provides generic parsing functionality for various output formats
type OutputParser struct {
	parsers map[string]func(string) (map[string]string, error)
}

// NewOutputParser creates a new output parser
func NewOutputParser() *OutputParser {
	parser := &OutputParser{
		parsers: make(map[string]func(string) (map[string]string, error)),
	}
	parser.registerDefaultParsers()
	return parser
}

// registerDefaultParsers registers the default parsing strategies
func (p *OutputParser) registerDefaultParsers() {
	p.parsers["key_value_space"] = p.parseKeyValueSpace
	p.parsers["key_value_equals"] = p.parseKeyValueEquals
	p.parsers["csv"] = p.parseCSV
	p.parsers["ssh_config"] = p.parseSSHConfig
}

// ParseSSHConfig parses SSH configuration output
func (p *OutputParser) ParseSSHConfig(output string) (map[string]string, error) {
	return p.parsers["ssh_config"](output)
}

// parseSSHConfig parses SSH config format (key value pairs separated by spaces).
// Handles multi-host output by only parsing the first Host block.
// If no Host blocks are found, parses all lines (single-VM output).
func (p *OutputParser) parseSSHConfig(output string) (map[string]string, error) {
	config := make(map[string]string)
	lines := strings.Split(output, "\n")
	foundHost := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect Host blocks - "Host <name>" lines start a new block
		if strings.HasPrefix(line, "Host ") {
			if foundHost {
				break
			}
			foundHost = true
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			key := parts[0]
			value := strings.TrimSpace(parts[1])
			config[key] = value
		}
	}

	return config, nil
}

// parseKeyValueSpace parses key-value pairs separated by spaces
func (p *OutputParser) parseKeyValueSpace(output string) (map[string]string, error) {
	result := make(map[string]string)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}

	return result, nil
}

// parseKeyValueEquals parses key-value pairs separated by equals
func (p *OutputParser) parseKeyValueEquals(output string) (map[string]string, error) {
	result := make(map[string]string)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}

	return result, nil
}

// parseCSV parses comma-separated values
func (p *OutputParser) parseCSV(output string) (map[string]string, error) {
	result := make(map[string]string)
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		for j, part := range parts {
			key := fmt.Sprintf("line_%d_col_%d", i, j)
			result[key] = strings.TrimSpace(part)
		}
	}

	return result, nil
}

// Global parser instance
var GlobalOutputParser = NewOutputParser()
