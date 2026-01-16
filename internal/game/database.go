package game

import (
	"errors"
	"io/fs"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jgabor/spela/internal/xdg"
)

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
		games = append(games, g)
	}
	return games
}

func (db *Database) GamesWithDLSS() []*Game {
	var games []*Game
	for _, g := range db.Games {
		if g.HasDLSS() || g.HasDLSSG() || g.HasDLSSD() {
			games = append(games, g)
		}
	}
	return games
}
