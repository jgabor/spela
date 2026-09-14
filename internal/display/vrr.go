// Package display temporarily configures the KDE session's primary display.
package display

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ApplyVRR captures the primary output's policy and returns its restoration.
// A cleanup may accompany an error because a failed setter can have applied
// state before reporting failure. Callers must retain that cleanup too.
func ApplyVRR(policy string) (func() error, error) {
	return applyVRR(policy, os.Getenv("XDG_CURRENT_DESKTOP"), os.Getenv("XDG_SESSION_TYPE"), runDoctor)
}

type doctorCommand func(...string) ([]byte, error)

func runDoctor(arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "kscreen-doctor", arguments...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("kscreen-doctor %s: %w", strings.Join(arguments, " "), err)
	}
	return output, nil
}

func applyVRR(policy, desktop, session string, run doctorCommand) (func() error, error) {
	if policy == "" || policy == "unset" {
		return nil, nil
	}
	if !validPolicy(policy) {
		return nil, fmt.Errorf("invalid VRR policy %q", policy)
	}
	if session != "wayland" || !containsKDE(desktop) {
		return nil, fmt.Errorf("VRR requires a KDE Wayland session")
	}
	outputs, err := discover(run)
	if err != nil {
		return nil, err
	}
	var primary *output
	for index := range outputs {
		candidate := &outputs[index]
		if candidate.priority == 1 {
			if primary != nil {
				return nil, fmt.Errorf("ambiguous KDE primary display")
			}
			primary = candidate
		}
	}
	if primary == nil || !primary.enabled || !primary.connected {
		return nil, fmt.Errorf("no connected, enabled KDE primary display")
	}
	if !validPolicy(primary.policy) {
		return nil, fmt.Errorf("primary display %s has unsupported or unreadable VRR policy %q", primary.name, primary.policy)
	}
	if primary.policy == policy {
		return nil, nil
	}
	original := *primary
	restore := func() error {
		current, err := discover(run)
		if err != nil {
			return err
		}
		for _, candidate := range current {
			if candidate.uuid == original.uuid {
				if !candidate.connected || !candidate.enabled || !validPolicy(candidate.policy) {
					return fmt.Errorf("original display %s is disconnected, disabled, or no longer supports VRR", original.name)
				}
				return setPolicy(run, original.uuid, original.policy)
			}
		}
		return fmt.Errorf("original display %s (%s) disconnected; VRR not restored", original.name, original.uuid)
	}
	err = setPolicy(run, original.uuid, policy)
	return restore, err
}

func setPolicy(run doctorCommand, uuid, policy string) error {
	data, err := run("output." + uuid + ".vrrpolicy." + policy)
	if err != nil {
		return err
	}
	// Plasma 6.5 doctor.cpp reports SetConfigOperation failure on stdout
	// but still exits zero. Do not mistake that for a successful mutation.
	if strings.Contains(strings.ToLower(string(data)), "applying config failed!") {
		return fmt.Errorf("kscreen-doctor: %s", strings.TrimSpace(string(data)))
	}
	return nil
}

func containsKDE(desktop string) bool {
	for _, name := range strings.Split(desktop, ":") {
		if strings.EqualFold(name, "KDE") {
			return true
		}
	}
	return false
}

func validPolicy(policy string) bool {
	return policy == "automatic" || policy == "always" || policy == "never"
}

type output struct {
	name, uuid, policy string
	priority           int
	enabled, connected bool
}

var (
	ansiColor  = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	outputUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// KDE 6.5+ doctor.cpp emits an unlocalized output block with UUID, priority,
// enabled/connected state and Vrr policy (or incapable). Unknown formats fail
// closed; unrelated mode, geometry and color lines are deliberately ignored.
func discover(run doctorCommand) ([]output, error) {
	data, err := run("-o")
	if err != nil {
		return nil, err
	}
	return parseOutputs(string(data))
}

func parseOutputs(text string) ([]output, error) {
	var outputs []output
	var current *output
	seen := map[string]bool{}
	properties := map[string]bool{}
	finish := func() error {
		if current != nil && (!properties["enabled"] || !properties["connected"] || !properties["priority"] || !properties["Vrr:"]) {
			return fmt.Errorf("incomplete KDE discovery for %s", current.name)
		}
		return nil
	}
	for _, line := range strings.Split(ansiColor.ReplaceAllString(text, ""), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "Output:" {
			if err := finish(); err != nil {
				return nil, err
			}
			if len(fields) != 4 || !outputUUID.MatchString(fields[3]) || fields[3] == "00000000-0000-0000-0000-000000000000" || seen[fields[3]] {
				return nil, fmt.Errorf("unsupported or ambiguous KDE output identity: %s", line)
			}
			seen[fields[3]] = true
			outputs = append(outputs, output{name: fields[2], uuid: fields[3]})
			current = &outputs[len(outputs)-1]
			properties = map[string]bool{}
			continue
		}
		if current == nil {
			continue
		}
		key := fields[0]
		switch key {
		case "enabled", "disabled":
			current.enabled = key == "enabled"
			key = "enabled"
		case "connected", "disconnected":
			current.connected = key == "connected"
			key = "connected"
		case "priority":
			if len(fields) != 2 {
				return nil, fmt.Errorf("unreadable KDE priority")
			}
			priority, err := strconv.Atoi(fields[1])
			if err != nil || priority < 0 {
				return nil, fmt.Errorf("unreadable KDE priority")
			}
			current.priority = priority
		case "Vrr:":
			if len(fields) != 2 {
				return nil, fmt.Errorf("unreadable KDE VRR policy")
			}
			current.policy = strings.ToLower(fields[1])
		default:
			continue
		}
		if properties[key] {
			return nil, fmt.Errorf("ambiguous KDE %s", key)
		}
		properties[key] = true
	}
	if err := finish(); err != nil {
		return nil, err
	}
	if len(outputs) == 0 {
		return nil, fmt.Errorf("no KDE outputs discovered")
	}
	return outputs, nil
}
