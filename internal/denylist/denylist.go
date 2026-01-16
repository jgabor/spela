package denylist

import (
	"errors"
	"io/fs"
	"os"

	"github.com/jgabor/spela/internal/xdg"
	"gopkg.in/yaml.v3"
)

type Entry struct {
	AppID  uint64 `yaml:"app_id"`
	Name   string `yaml:"name"`
	Reason string `yaml:"reason"`
}

type DenyList struct {
	Entries []Entry `yaml:"entries"`
}

type Overrides struct {
	Allowed []uint64 `yaml:"allowed"`
	Denied  []Entry  `yaml:"denied"`
}

var defaultDenyList = DenyList{
	Entries: []Entry{
		{AppID: 1172470, Name: "Apex Legends", Reason: "Easy Anti-Cheat"},
		{AppID: 1938090, Name: "Call of Duty: HQ", Reason: "RICOCHET Anti-Cheat"},
		{AppID: 1517290, Name: "Battlefield 2042", Reason: "Easy Anti-Cheat"},
		{AppID: 1063730, Name: "New World", Reason: "Easy Anti-Cheat"},
		{AppID: 1599340, Name: "Lost Ark", Reason: "Easy Anti-Cheat"},
		{AppID: 594650, Name: "Hunt: Showdown", Reason: "Easy Anti-Cheat"},
		{AppID: 578080, Name: "PUBG: Battlegrounds", Reason: "Anti-Cheat"},
		{AppID: 1085660, Name: "Destiny 2", Reason: "BattlEye"},
		{AppID: 252490, Name: "Rust", Reason: "Easy Anti-Cheat"},
		{AppID: 1623660, Name: "Escape from Tarkov", Reason: "BattlEye"},
	},
}

func listsDir() string {
	return xdg.ConfigPath("lists")
}

func denyListPath() string {
	return xdg.ConfigPath("lists", "denylist.yaml")
}

func overridesPath() string {
	return xdg.ConfigPath("lists", "overrides.yaml")
}

func EnsureListsDir() error {
	return os.MkdirAll(listsDir(), 0755)
}

func LoadDenyList() (*DenyList, error) {
	path := denyListPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &defaultDenyList, nil
		}
		return nil, err
	}

	var list DenyList
	if err := yaml.Unmarshal(data, &list); err != nil {
		return nil, err
	}

	return &list, nil
}

func LoadOverrides() (*Overrides, error) {
	path := overridesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Overrides{}, nil
		}
		return nil, err
	}

	var overrides Overrides
	if err := yaml.Unmarshal(data, &overrides); err != nil {
		return nil, err
	}

	return &overrides, nil
}

func SaveOverrides(o *Overrides) error {
	if err := EnsureListsDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(o)
	if err != nil {
		return err
	}

	return os.WriteFile(overridesPath(), data, 0644)
}

func IsDenied(appID uint64) (bool, string) {
	overrides, _ := LoadOverrides()
	if overrides != nil {
		for _, id := range overrides.Allowed {
			if id == appID {
				return false, ""
			}
		}
		for _, e := range overrides.Denied {
			if e.AppID == appID {
				return true, e.Reason
			}
		}
	}

	list, _ := LoadDenyList()
	if list != nil {
		for _, e := range list.Entries {
			if e.AppID == appID {
				return true, e.Reason
			}
		}
	}

	return false, ""
}

func Allow(appID uint64) error {
	overrides, err := LoadOverrides()
	if err != nil {
		return err
	}

	for _, id := range overrides.Allowed {
		if id == appID {
			return nil
		}
	}

	overrides.Allowed = append(overrides.Allowed, appID)
	return SaveOverrides(overrides)
}

func Deny(appID uint64, name, reason string) error {
	overrides, err := LoadOverrides()
	if err != nil {
		return err
	}

	for i, e := range overrides.Denied {
		if e.AppID == appID {
			overrides.Denied[i].Reason = reason
			return SaveOverrides(overrides)
		}
	}

	overrides.Denied = append(overrides.Denied, Entry{
		AppID:  appID,
		Name:   name,
		Reason: reason,
	})

	return SaveOverrides(overrides)
}

func RemoveAllow(appID uint64) error {
	overrides, err := LoadOverrides()
	if err != nil {
		return err
	}

	for i, id := range overrides.Allowed {
		if id == appID {
			overrides.Allowed = append(overrides.Allowed[:i], overrides.Allowed[i+1:]...)
			return SaveOverrides(overrides)
		}
	}

	return nil
}

func RemoveDeny(appID uint64) error {
	overrides, err := LoadOverrides()
	if err != nil {
		return err
	}

	for i, e := range overrides.Denied {
		if e.AppID == appID {
			overrides.Denied = append(overrides.Denied[:i], overrides.Denied[i+1:]...)
			return SaveOverrides(overrides)
		}
	}

	return nil
}
