package slurm

import (
	"fmt"
	"slices"
	"strings"
)

var nodeBases = map[string]bool{
	"UNKNOWN": true, "DOWN": true, "IDLE": true, "ALLOCATED": true,
	"MIXED": true, "FUTURE": true, "INVALID": true,
}

var nodeFlagNames = map[string]bool{
	"EXTERNAL": true, "RESERVED": true, "UNDRAIN": true, "CLOUD": true,
	"RESUME": true, "DRAIN": true, "COMPLETING": true, "NOT_RESPONDING": true,
	"POWERED_DOWN": true, "FAIL": true, "POWERING_UP": true, "MAINTENANCE": true,
	"REBOOT_REQUESTED": true, "REBOOT_CANCELED": true, "POWERING_DOWN": true,
	"DYNAMIC_FUTURE": true, "REBOOT_ISSUED": true, "PLANNED": true,
	"INVALID_REG": true, "POWER_DOWN": true, "POWER_UP": true, "POWER_DRAIN": true,
	"DYNAMIC_NORM": true, "BLOCKED": true,
}

var nodeFlagAliases = map[string][]string{
	"POWER_DOWN_ASAP":  {"POWER_DOWN", "POWER_DRAIN"},
	"POWER_DOWN_FORCE": {"POWER_DOWN", "POWERED_DOWN"},
}

type NodeState struct {
	Base  string
	Flags map[string]bool
	Raw   []string
}

func ParseNodeState(values []string) (NodeState, error) {
	s := NodeState{Base: "UNKNOWN", Flags: map[string]bool{}, Raw: values}
	baseSeen := false
	for _, v := range values {
		if nodeBases[v] {
			if baseSeen {
				return NodeState{}, fmt.Errorf("state: two base states in %q", values)
			}
			s.Base = v
			baseSeen = true
			continue
		}
		if nodeFlagNames[v] {
			s.Flags[v] = true
			continue
		}
		aliases, ok := nodeFlagAliases[v]
		if !ok {
			return NodeState{}, fmt.Errorf("state: unknown value %q", v)
		}
		for _, f := range aliases {
			s.Flags[f] = true
		}
	}
	if !baseSeen {
		return NodeState{}, fmt.Errorf("state: no base state in %q", values)
	}
	return s, nil
}

func (s NodeState) has(flag string) bool {
	return s.Flags[flag]
}

type NodeClass int

const (
	NodeDown NodeClass = iota
	NodeDrain
	NodeMixed
	NodeAllocated
	NodeIdle
	NodeOther
)

var flagsThatMakeIdleOther = []string{
	"POWERED_DOWN", "POWERING_UP", "POWERING_DOWN", "MAINTENANCE",
	"REBOOT_REQUESTED", "REBOOT_ISSUED", "RESERVED",
}

func (s NodeState) Class() NodeClass {
	switch {
	case s.Down():
		return NodeDown
	case s.InDrain():
		return NodeDrain
	case s.Base == "MIXED":
		return NodeMixed
	case s.Base == "ALLOCATED" || s.has("COMPLETING"):
		return NodeAllocated
	case s.Base == "IDLE" && !slices.ContainsFunc(flagsThatMakeIdleOther, s.has):
		return NodeIdle
	}
	return NodeOther
}

func (s NodeState) InDrain() bool {
	return s.has("DRAIN")
}

func (s NodeState) Resumable() bool {
	return s.InDrain() || s.Class() == NodeDown
}

func (s NodeState) RebootRequested() bool {
	return s.has("REBOOT_REQUESTED")
}

func (s NodeState) only(flag string) bool {
	return s.Base == "UNKNOWN" && len(s.Flags) == 1 && s.Flags[flag]
}

func (s NodeState) suffix() string {
	switch {
	case s.has("MAINTENANCE"):
		return "$"
	case s.has("REBOOT_ISSUED"):
		return "^"
	case s.has("REBOOT_REQUESTED"):
		return "@"
	case s.has("POWERING_UP"):
		return "#"
	case s.has("POWERING_DOWN"):
		return "%"
	case s.has("POWERED_DOWN"):
		return "~"
	case s.has("POWER_DOWN"):
		return "!"
	case s.has("NOT_RESPONDING"):
		return "*"
	}
	return ""
}

func (s NodeState) Down() bool {
	return s.Base == "DOWN" || s.has("FAIL") || s.has("NOT_RESPONDING")
}

func (s NodeState) Draining() bool {
	return s.has("DRAIN") && (s.has("COMPLETING") || s.Base == "ALLOCATED" || s.Base == "MIXED")
}

func (s NodeState) Label() string {
	return strings.ToLower(s.upperLabel())
}

func (s NodeState) upperLabel() string {
	busy := s.Base == "ALLOCATED" || s.Base == "MIXED"
	noResp := s.has("NOT_RESPONDING")
	if s.Base == "INVALID" {
		return "INVALID"
	}
	if s.has("INVALID_REG") {
		return "INVAL"
	}
	if s.has("MAINTENANCE") && !s.has("DRAIN") && !busy && s.Base != "DOWN" {
		if noResp {
			return "MAINT*"
		}
		return "MAINT"
	}
	if (s.has("REBOOT_REQUESTED") || s.has("REBOOT_ISSUED")) && !busy {
		switch {
		case s.has("REBOOT_ISSUED"):
			return "REBOOT^"
		case noResp:
			return "REBOOT*"
		}
		return "REBOOT"
	}
	if s.has("DRAIN") {
		if s.Draining() {
			return "DRAINING" + s.suffix()
		}
		return "DRAINED" + s.suffix()
	}
	if s.has("FAIL") {
		star := ""
		if noResp {
			star = "*"
		}
		if s.has("COMPLETING") || busy {
			return "FAILING" + star
		}
		return "FAIL" + star
	}
	exact := []struct{ flag, label string }{
		{"REBOOT_ISSUED", "REBOOT_ISSUED"}, {"REBOOT_CANCELED", "CANCEL_REBOOT"},
		{"CLOUD", "CLOUD"}, {"EXTERNAL", "EXTERNAL"}, {"POWER_DOWN", "POWER_DOWN"},
		{"POWER_UP", "POWER_UP"}, {"POWERING_DOWN", "POWERING_DOWN"},
		{"POWERED_DOWN", "POWERED_DOWN"}, {"POWERING_UP", "POWERING_UP"},
		{"UNDRAIN", "UNDRAIN"}, {"DYNAMIC_FUTURE", "DYNAMIC_FUTURE"},
		{"DYNAMIC_NORM", "DYNAMIC_NORM"},
	}
	for _, e := range exact {
		if s.only(e.flag) {
			return e.label
		}
	}
	switch s.Base {
	case "DOWN":
		return "DOWN" + s.suffix()
	case "ALLOCATED":
		if sfx := s.suffix(); sfx != "" {
			return "ALLOCATED" + sfx
		}
		if s.has("COMPLETING") {
			return "ALLOCATED+"
		}
		return "ALLOCATED"
	}
	if s.has("COMPLETING") {
		return "COMPLETING" + s.suffix()
	}
	switch s.Base {
	case "IDLE":
		if sfx := s.suffix(); sfx != "" {
			return "IDLE" + sfx
		}
		switch {
		case s.has("RESERVED"):
			return "RESERVED"
		case s.has("BLOCKED"):
			return "BLOCKED"
		case s.has("PLANNED"):
			return "PLANNED"
		}
		return "IDLE"
	case "MIXED":
		if sfx := s.suffix(); sfx != "" {
			return "MIXED" + sfx
		}
		if s.has("PLANNED") {
			return "MIXED-"
		}
		return "MIXED"
	case "FUTURE":
		return "FUTURE" + s.suffix()
	}
	if s.has("RESUME") {
		return "RESUME"
	}
	if noResp {
		return "UNKNOWN*"
	}
	return "UNKNOWN"
}
