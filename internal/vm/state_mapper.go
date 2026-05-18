// Copyright Ricardo Oliveira 2025.
// SPDX-License-Identifier: MPL-2.0

package vm

import (
	"fmt"
	"strings"

	"github.com/vagrant-mcp/server/internal/core"
)

// StateMapper handles mapping between different state representations
type StateMapper struct {
	vagrantStateMap map[string]core.VMState
	parseStrategies map[string]func(string) (core.VMState, error)
}

// NewStateMapper creates a new state mapper
func NewStateMapper() *StateMapper {
	mapper := &StateMapper{
		vagrantStateMap: make(map[string]core.VMState),
		parseStrategies: make(map[string]func(string) (core.VMState, error)),
	}
	mapper.registerDefaultMappings()
	mapper.registerDefaultStrategies()
	return mapper
}

// registerDefaultMappings registers the default Vagrant state mappings
func (m *StateMapper) registerDefaultMappings() {
	m.vagrantStateMap["running"] = core.Running
	m.vagrantStateMap["poweroff"] = core.Stopped
	m.vagrantStateMap["shutoff"] = core.Stopped     // libvirt
	m.vagrantStateMap["aborted"] = core.Stopped
	m.vagrantStateMap["saved"] = core.Suspended
	m.vagrantStateMap["paused"] = core.Suspended     // libvirt
	m.vagrantStateMap["suspended"] = core.Suspended  // Hyper-V / Azure
	m.vagrantStateMap["not_created"] = core.NotCreated
	m.vagrantStateMap["preparing"] = core.Unknown    // VMware
	m.vagrantStateMap["stuck"] = core.Error          // VirtualBox
	m.vagrantStateMap["inaccessible"] = core.Error   // VirtualBox
}

// registerDefaultStrategies registers the default parsing strategies
func (m *StateMapper) registerDefaultStrategies() {
	m.parseStrategies["vagrant_machine_readable"] = m.parseVagrantMachineReadable
	m.parseStrategies["vagrant_human_readable"] = m.parseVagrantHumanReadable
}

// ParseVagrantState parses Vagrant state output using the machine-readable format
func (m *StateMapper) ParseVagrantState(output string) (core.VMState, error) {
	return m.parseStrategies["vagrant_machine_readable"](output)
}

// parseVagrantMachineReadable parses machine-readable Vagrant output
func (m *StateMapper) parseVagrantMachineReadable(output string) (core.VMState, error) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) >= 4 && parts[2] == "state" {
			if state, exists := m.vagrantStateMap[parts[3]]; exists {
				return state, nil
			}
			// If not found in map, return Unknown instead of erroring
			// Unrecognized provider states should not block all operations
			return core.Unknown, nil
		}
	}

	return core.Error, fmt.Errorf("could not determine VM state from output: %s", output)
}

// parseVagrantHumanReadable parses human-readable Vagrant output (for future use)
func (m *StateMapper) parseVagrantHumanReadable(output string) (core.VMState, error) {
	lines := strings.Split(strings.ToLower(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		for vagrantState, vmState := range m.vagrantStateMap {
			if strings.Contains(line, vagrantState) {
				return vmState, nil
			}
		}
	}

	return core.Unknown, fmt.Errorf("could not parse human-readable state: %s", output)
}

// Global state mapper instance
var GlobalStateMapper = NewStateMapper()
