package overlay

// ParsePosition converts a position string to the wire format uint8.
//
//   - 0 = top-left (default)
//   - 1 = top-right
//   - 2 = bottom-left
//   - 3 = bottom-right
func ParsePosition(s string) uint8 {
	switch s {
	case "top-right":
		return 1
	case "bottom-left":
		return 2
	case "bottom-right":
		return 3
	default:
		return 0 // top-left
	}
}
