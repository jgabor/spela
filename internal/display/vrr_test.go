package display

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const (
	primaryUUID   = "11111111-2222-3333-4444-555555555555"
	secondaryUUID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

// Minimal blocks retain the discovery fields emitted by KDE 6.5 and 6.7.
const (
	primaryFixture   = "Output: 7 DP-3 " + primaryUUID + "\n\tenabled\n\tconnected\n\tpriority 1\n\tVrr: Automatic\n"
	secondaryFixture = "Output: 1 HDMI-A-1 " + secondaryUUID + "\n\tenabled\n\tconnected\n\tpriority 2\n\tVrr: Never\n"
)

func TestApplyVRRPoliciesAndOriginalOutputRestore(t *testing.T) {
	for _, policy := range []string{"", "unset", "automatic", "always", "never"} {
		t.Run("policy="+policy, func(t *testing.T) {
			var calls []string
			fixture := secondaryFixture + primaryFixture
			run := func(arguments ...string) ([]byte, error) {
				calls = append(calls, strings.Join(arguments, " "))
				return []byte(fixture), nil
			}
			restore, err := applyVRR(policy, "KDE", "wayland", run)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			switch policy {
			case "", "unset":
				if restore != nil {
					t.Fatal("no-op returned cleanup")
				}
			case "automatic":
				want = []string{"-o"}
				if restore != nil {
					t.Fatal("unchanged policy returned cleanup")
				}
			default:
				if restore == nil {
					t.Fatal("missing cleanup")
				}
				// Primary changes while the game runs. Restore the old UUID,
				// not output 1, the new primary or its current connector name.
				fixture = strings.ReplaceAll(secondaryFixture, "priority 2", "priority 1") + strings.ReplaceAll(primaryFixture, "priority 1", "priority 2")
				if err := restore(); err != nil {
					t.Fatal(err)
				}
				want = []string{"-o", "output." + primaryUUID + ".vrrpolicy." + policy, "-o", "output." + primaryUUID + ".vrrpolicy.automatic"}
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls = %v, want %v", calls, want)
			}
		})
	}
}

func TestApplyVRRRejectsUnsafeDiscoveryWithoutMutation(t *testing.T) {
	for name, fixture := range map[string]string{
		"empty": "", "garbage": "not KDE", "old format without UUID": strings.ReplaceAll(primaryFixture, " "+primaryUUID, ""),
		"unsupported":          strings.ReplaceAll(primaryFixture, "Automatic", "incapable"),
		"unknown policy":       strings.ReplaceAll(primaryFixture, "Automatic", "Adaptive"),
		"missing policy":       strings.ReplaceAll(primaryFixture, "\tVrr: Automatic\n", ""),
		"missing priority":     strings.ReplaceAll(primaryFixture, "\tpriority 1\n", ""),
		"invalid priority":     strings.ReplaceAll(primaryFixture, "priority 1", "priority unknown"),
		"missing connected":    strings.ReplaceAll(primaryFixture, "\tconnected\n", ""),
		"disabled primary":     strings.ReplaceAll(primaryFixture, "enabled", "disabled"),
		"disconnected primary": strings.ReplaceAll(primaryFixture, "connected", "disconnected"),
		"no primary":           secondaryFixture,
		"two primaries":        primaryFixture + strings.ReplaceAll(secondaryFixture, "priority 2", "priority 1"),
		"duplicate UUID":       primaryFixture + primaryFixture,
		"null UUID":            strings.ReplaceAll(primaryFixture, primaryUUID, "00000000-0000-0000-0000-000000000000"),
		"duplicate policy":     primaryFixture + "\tVrr: Never\n",
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			restore, err := applyVRR("always", "KDE", "wayland", func(arguments ...string) ([]byte, error) {
				calls++
				if !reflect.DeepEqual(arguments, []string{"-o"}) {
					t.Fatal("unsafe mutation", arguments)
				}
				return []byte(fixture), nil
			})
			if err == nil || restore != nil || calls != 1 {
				t.Fatalf("restore=%v err=%v calls=%d", restore != nil, err, calls)
			}
		})
	}
}

func TestVRRSessionAndDiscoveryFailure(t *testing.T) {
	for _, test := range []struct{ policy, desktop, session string }{
		{"adaptive", "KDE", "wayland"}, {"always", "GNOME", "wayland"}, {"always", "KDE", "x11"}, {"always", "", ""},
	} {
		_, err := applyVRR(test.policy, test.desktop, test.session, func(...string) ([]byte, error) { t.Fatal("unexpected command"); return nil, nil })
		if err == nil {
			t.Fatalf("accepted %+v", test)
		}
	}
	for _, policy := range []string{"", "unset"} {
		if _, err := applyVRR(policy, "", "", func(...string) ([]byte, error) { t.Fatal("unexpected command"); return nil, nil }); err != nil {
			t.Fatal(err)
		}
	}
	_, err := applyVRR("always", "KDE", "wayland", func(...string) ([]byte, error) { return nil, errors.New("tool unavailable") })
	if err == nil || !strings.Contains(err.Error(), "tool unavailable") {
		t.Fatal(err)
	}
}

func TestVRRRestoreFailuresNeverTargetReplacement(t *testing.T) {
	for _, failure := range []string{"apply", "discover", "disconnected", "replacement", "disabled", "unsupported", "restore"} {
		t.Run(failure, func(t *testing.T) {
			calls := 0
			restore, err := applyVRR("always", "KDE", "wayland", func(arguments ...string) ([]byte, error) {
				calls++
				switch calls {
				case 1:
					return []byte(primaryFixture), nil
				case 2:
					if failure == "apply" {
						return nil, errors.New("apply failed after possible mutation")
					}
				case 3:
					switch failure {
					case "discover":
						return nil, errors.New("discovery failed")
					case "disconnected":
						return []byte(secondaryFixture), nil
					case "replacement":
						return []byte(strings.ReplaceAll(primaryFixture, primaryUUID, secondaryUUID)), nil
					case "disabled":
						return []byte(strings.ReplaceAll(primaryFixture, "enabled", "disabled")), nil
					case "unsupported":
						return []byte(strings.ReplaceAll(primaryFixture, "Automatic", "incapable")), nil
					}
					return []byte(primaryFixture), nil
				case 4:
					if arguments[0] != "output."+primaryUUID+".vrrpolicy.automatic" {
						t.Fatal(arguments)
					}
					if failure == "restore" {
						return nil, errors.New("restore failed")
					}
				default:
					t.Fatal("unexpected call")
				}
				return nil, nil
			})
			if (err != nil) != (failure == "apply") || restore == nil {
				t.Fatalf("apply err=%v cleanup=%v", err, restore != nil)
			}
			err = restore()
			if (err != nil) != (failure != "apply") {
				t.Fatalf("restore err=%v", err)
			}
			if failure != "apply" && failure != "restore" && calls != 3 {
				t.Fatal("mutated after unsafe rediscovery")
			}
		})
	}
}

func TestParseKDEColoredOutput(t *testing.T) {
	fixture := strings.ReplaceAll(primaryFixture, "Output:", "\x1b[01;32mOutput: \x1b[0;0m") + "\tGeometry: 0,0 2560x1440\n\tHDR: enabled\n"
	outputs, err := parseOutputs(fixture)
	if err != nil || len(outputs) != 1 || outputs[0].policy != "automatic" {
		t.Fatalf("%+v %v", outputs, err)
	}
}

func TestVRRReportsZeroExitConfigFailure(t *testing.T) {
	for _, failure := range []string{"apply", "restore"} {
		t.Run(failure, func(t *testing.T) {
			setCount := 0
			restore, err := applyVRR("always", "KDE", "wayland", func(arguments ...string) ([]byte, error) {
				if arguments[0] == "-o" {
					return []byte(primaryFixture), nil
				}
				setCount++
				if (failure == "apply" && setCount == 1) || (failure == "restore" && setCount == 2) {
					return []byte("applying config failed! Backend rejected configuration\n"), nil
				}
				return nil, nil
			})
			if (err != nil) != (failure == "apply") || restore == nil {
				t.Fatalf("apply = %v, cleanup = %v", err, restore != nil)
			}
			if err := restore(); (err != nil) != (failure == "restore") {
				t.Fatalf("restore = %v", err)
			}
		})
	}
}
