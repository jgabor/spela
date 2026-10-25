//go:build e2e

package release

import (
	"net"
	"net/url"
	"os"
)

// End-to-end builds may point the release lookup at a local stub server.
func init() {
	value := os.Getenv("SPELA_E2E_RELEASES_URL")
	if value == "" {
		return
	}
	if parsed, err := url.Parse(value); err == nil {
		if ip := net.ParseIP(parsed.Hostname()); ip != nil && ip.IsLoopback() {
			TagsURL = value
		}
	}
}
