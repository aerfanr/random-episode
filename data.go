package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func closeDB() {
	db.Close()
}

type Episode struct {
	season  int
	episode int
	number  int
}

type Show struct {
	name          string
	episodeCount  int
	seasonCount   int
	seasonLengths []int
}

func newShow() Show {
	return Show{}
}

func (s Show) save() error {
	if db == nil {
		return fmt.Errorf("No db connection")
	}

	sl := ""
	for _, l := range s.seasonLengths {
		sl += fmt.Sprintf("%d,", l)
	}

	stmt := `INSERT INTO series (name, season_lengths) VALUES (?, ?);`
	fmt.Println(stmt)
	_, err := db.Exec(stmt, s.name, sl)
	if err != nil {
		return err
	}

	return nil
}

func (s Show) getEpisode() (Episode, error) {
	for i := 0; i < 50; i++ {
		n := rand.Intn(s.episodeCount)

		stmt := `SELECT number FROM episodes WHERE series = ? AND number = ? AND watched = 1;`
		rows := db.QueryRow(stmt, s.name, n)
		if rows.Scan() == sql.ErrNoRows {
			m := n
			for j, l := range s.seasonLengths {
				if m < l {
					return Episode{
						season:  j + 1,
						episode: m + 1,
						number:  n,
					}, nil
				}
				m -= l
			}
		}
	}

	return Episode{}, fmt.Errorf("No episode found")
}

func (s Show) watchEpisode(e Episode) error {
	stmt := `INSERT INTO episodes (series, number, watched) VALUES (?, ?, 1);`
	_, err := db.Exec(stmt, s.name, e.number)
	return err
}

func showFromRow(rows *sql.Rows) (Show, error) {
	var name string
	var seasonLengths string
	err := rows.Scan(&name, &seasonLengths)
	if err != nil {
		return Show{}, err
	}

	s := newShow()
	s.name = name
	for _, l := range strings.Split(seasonLengths, ",") {
		n, err := strconv.Atoi(l)
		if err == nil {
			s.seasonLengths = append(s.seasonLengths, n)
			s.episodeCount += n
		}
	}

	return s, nil
}

// Connect to databse and return a Msg current shows from the db
func connectDB() tea.Msg {
	dbDir := os.Getenv("XDG_DATA_HOME")
	if dbDir == "" {
		dbDir = os.Getenv("HOME") + "/.local/share"
	}
	dbDir = dbDir + "/random-episode"

	os.MkdirAll(dbDir, 0755)

	// Connect sqlite db
	var err error
	db, err = sql.Open("sqlite3", dbDir+"/data.db")
	if err != nil {
		log.Fatal(err)
	}

	err = alterDB()
	if err != nil {
		log.Fatal(err)
	}

	return readShows()
}

// Check database schema version and apply new changes if needed
func alterDB() error {
	stmt := "PRAGMA user_version"
	var version int
	err := db.QueryRow(stmt).Scan(&version)
	if err != nil {
		return err
	}

	if version < 1 {
		stmt = `CREATE TABLE IF NOT EXISTS series
		(name text, season_lengths text);`
		_, err = db.Exec(stmt)
		if err != nil {
			return err
		}

		stmt = `CREATE TABLE IF NOT EXISTS episodes
		(series text, number int, watched int);`
		_, err = db.Exec(stmt)
		if err != nil {
			return err
		}
	}

	if version < 1 { // MAKE SURE YOU UPDATE THIS ON CHANGE
		stmt := "PRAGMA user_version = 1" // MAKE SURE YOU UPDATE THIS ON CHANGE
		_, err = db.Exec(stmt)
		if err != nil {
			return err
		}
	}

	return nil
}

func readShows() tea.Msg {
	stmt := `SELECT name, season_lengths FROM series;`
	rows, err := db.Query(stmt)
	if err != nil {
		log.Fatal(err)
	}

	shows := []Show{}
	for rows.Next() {
		s, err := showFromRow(rows)
		if err != nil {
			log.Print(err)
		}
		shows = append(shows, s)
	}

	return showsLoaded{
		shows: shows,
	}
}
