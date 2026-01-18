package game

import (
	"errors"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/xdg"
)

// Tool name patterns for filtering non-game entries from the database.
var toolNamePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^proton(\s|$)`),
	regexp.MustCompile(`(?i)^steam\s+linux\s+runtime`),
	regexp.MustCompile(`(?i)^steamworks`),
	regexp.MustCompile(`(?i)redistributable`),
	regexp.MustCompile(`(?i)^steam\s+controller`),
}

func isToolName(name string) bool {
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

func (db *Database) Save() error {
	if _, err := xdg.EnsureDataHome(); err != nil {
		return err
	}

	db.UpdatedAt = time.Now()

	data, err := yaml.Marshal(db)
	if err != nil {
		return err
	}

	path := xdg.DataPath("games.yaml")
	return os.WriteFile(path, data, 0o644)
}

func (db *Database) AddGame(game *Game) {
	db.Games[game.AppID] = game
}

func (db *Database) GetGame(appID uint64) *Game {
	return db.Games[appID]
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
		if isToolName(g.Name) {
			continue
		}
		games = append(games, g)
	}
	return games
}

func (db *Database) GamesWithDLSS() []*Game {
	var games []*Game
	for _, g := range db.Games {
		if isToolName(g.Name) {
			continue
		}
		if g.HasDLSS() || g.HasDLSSG() || g.HasDLSSD() {
			games = append(games, g)
		}
	}
	return games
}
