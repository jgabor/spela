package game

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/xdg"
)

// toolNamePatterns matches names of Steam tools (Proton, Runtimes, SDKs)
// that should not appear in game lists.
var toolNamePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^proton(\s|$)`),
	regexp.MustCompile(`(?i)^steam\s+linux\s+runtime`),
	regexp.MustCompile(`(?i)^steamworks`),
	regexp.MustCompile(`(?i)redistributable`),
	regexp.MustCompile(`(?i)^steam\s+controller`),
}

// IsToolName reports whether name matches a known Steam tool pattern.
func IsToolName(name string) bool {
	name = strings.TrimSpace(name)
	for _, pattern := range toolNamePatterns {
		if pattern.MatchString(name) {
			return true
		}
	}
	return false
}

type Database struct {
	Games     map[uint64]*Game `yaml:"games"`
	UpdatedAt time.Time        `yaml:"updated_at"`
}

var databaseMutex sync.Mutex

// Transaction serializes a fresh read-modify-save cycle across goroutines and
// processes. The callback reports whether its changes need persistence and
// must not call Transaction recursively.
func Transaction(update func(*Database) (bool, error)) (*Database, error) {
	databaseMutex.Lock()
	defer databaseMutex.Unlock()

	path := filepath.Join(xdg.RuntimeDir(), "games.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }()

	database, err := LoadDatabase()
	if err != nil {
		return nil, fmt.Errorf("load game database: %w", err)
	}
	changed, err := update(database)
	if err != nil || !changed {
		return database, err
	}
	if err := database.save(); err != nil {
		return database, err
	}
	return database, nil
}

func LoadDatabase() (*Database, error) {
	path := xdg.DataPath("games.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Database{Games: make(map[uint64]*Game)}, nil
		}
		return nil, err
	}

	var db Database
	if err := yaml.Unmarshal(data, &db); err != nil {
		return nil, err
	}

	if db.Games == nil {
		db.Games = make(map[uint64]*Game)
	}

	return &db, nil
}

func (db *Database) save() error {
	if _, err := xdg.EnsureDataHome(); err != nil {
		return err
	}

	db.UpdatedAt = time.Now()

	data, err := yaml.Marshal(db)
	if err != nil {
		return err
	}

	path := xdg.DataPath("games.yaml")
	temporary, err := os.CreateTemp(filepath.Dir(path), ".games-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func (db *Database) AddGame(game *Game) {
	db.Games[game.AppID] = game
}

func (db *Database) GetGame(appID uint64) *Game {
	return db.Games[appID]
}

// FindGame looks up a game by ID (if query is numeric) or name.
func (db *Database) FindGame(query string) *Game {
	if appID, err := strconv.ParseUint(query, 10, 64); err == nil {
		if g := db.GetGame(appID); g != nil {
			return g
		}
	}
	return db.GetGameByName(query)
}

func (db *Database) GetGameByName(name string) *Game {
	for _, g := range db.Games {
		if g.Name == name {
			return g
		}
	}
	return nil
}

func (db *Database) List() []*Game {
	games := make([]*Game, 0, len(db.Games))
	for _, g := range db.Games {
		if IsToolName(g.Name) {
			continue
		}
		games = append(games, g)
	}
	return games
}

func (db *Database) GamesWithDLSS() []*Game {
	games := make([]*Game, 0, len(db.Games))
	for _, g := range db.Games {
		if IsToolName(g.Name) {
			continue
		}
		if g.HasDLSS() || g.HasDLSSG() || g.HasDLSSD() {
			games = append(games, g)
		}
	}
	return games
}
