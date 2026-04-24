package commands

import "fmt"

// parseBoolFlag parses human-facing bool forms accepted by profile set commands.
// Unknown values return an error so callers never silently store false.
func parseBoolFlag(raw string) (bool, error) {
	switch raw {
	case "true", "True", "TRUE", "1", "yes", "on":
		return true, nil
	case "false", "False", "FALSE", "0", "no", "off":
		return false, nil
	}
	return false, fmt.Errorf("invalid bool %q (use true/false)", raw)
}
