package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

type Card struct {
	ID             int64
	Path           string
	Title          string
	Tag            string
	FirstIndexed   time.Time
	Stability      float64
	Difficulty     float64
	ElapsedDays    int
	ScheduledDays  int
	Reps           int
	Lapses         int
	State          string
	LastReview     time.Time
	NextDue        time.Time
}

func UpsertCard(db *sql.DB, c Card) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO cards(path,title,tag,first_indexed,stability,difficulty,
		                  elapsed_days,scheduled_days,reps,lapses,state,last_review,next_due)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET
		  title=excluded.title,
		  tag=excluded.tag
	`, c.Path, c.Title, c.Tag, c.FirstIndexed, c.Stability, c.Difficulty,
		c.ElapsedDays, c.ScheduledDays, c.Reps, c.Lapses, c.State,
		c.LastReview, c.NextDue)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetCardByPath(db *sql.DB, path string) (*Card, error) {
	row := db.QueryRow(`SELECT id,path,title,tag,first_indexed,stability,difficulty,
		elapsed_days,scheduled_days,reps,lapses,state,last_review,next_due
		FROM cards WHERE path=?`, path)
	return scanCard(row)
}

func GetDueCards(db *sql.DB, keywords []string) ([]Card, error) {
	now := time.Now().Format("2006-01-02")
	query := `SELECT id,path,title,tag,first_indexed,stability,difficulty,
		elapsed_days,scheduled_days,reps,lapses,state,last_review,next_due
		FROM cards WHERE (next_due <= ? OR next_due IS NULL OR next_due = '')
		ORDER BY
		  CASE WHEN reps = 0 THEN 0 ELSE 1 END ASC,
		  CASE WHEN reps = 0 THEN first_indexed END DESC,
		  next_due ASC`
	rows, err := db.Query(query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		if len(keywords) > 0 {
			content, err := readFile(c.Path)
			if err != nil {
				continue
			}
			lower := strings.ToLower(content)
			match := true
			for _, kw := range keywords {
				if !strings.Contains(lower, strings.ToLower(kw)) {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		cards = append(cards, *c)
	}
	return cards, rows.Err()
}

func UpdateCardSchedule(db *sql.DB, c Card) error {
	_, err := db.Exec(`UPDATE cards SET stability=?,difficulty=?,elapsed_days=?,
		scheduled_days=?,reps=?,lapses=?,state=?,last_review=?,next_due=?
		WHERE id=?`,
		c.Stability, c.Difficulty, c.ElapsedDays, c.ScheduledDays,
		c.Reps, c.Lapses, c.State, c.LastReview, c.NextDue, c.ID)
	return err
}

func UpdateCardPath(db *sql.DB, id int64, newPath string) error {
	_, err := db.Exec(`UPDATE cards SET path=? WHERE id=?`, newPath, id)
	return err
}

func DeleteCard(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM cards WHERE id=?`, id)
	return err
}

func ListAllCards(db *sql.DB) ([]Card, error) {
	rows, err := db.Query(`SELECT id,path,title,tag,first_indexed,stability,difficulty,
		elapsed_days,scheduled_days,reps,lapses,state,last_review,next_due
		FROM cards ORDER BY next_due ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, *c)
	}
	return cards, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCard(s scanner) (*Card, error) {
	var c Card
	var firstIndexed, lastReview, nextDue sql.NullString
	err := s.Scan(&c.ID, &c.Path, &c.Title, &c.Tag, &firstIndexed,
		&c.Stability, &c.Difficulty, &c.ElapsedDays, &c.ScheduledDays,
		&c.Reps, &c.Lapses, &c.State, &lastReview, &nextDue)
	if err != nil {
		return nil, err
	}
	const layout = "2006-01-02T15:04:05Z07:00"
	parseTime := func(s sql.NullString) time.Time {
		if !s.Valid || s.String == "" {
			return time.Time{}
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05 -0700 MST", "2006-01-02", layout} {
			if t, err := time.Parse(layout, s.String); err == nil {
				return t
			}
		}
		return time.Time{}
	}
	_ = layout
	c.FirstIndexed = parseTime(firstIndexed)
	c.LastReview = parseTime(lastReview)
	c.NextDue = parseTime(nextDue)
	return &c, nil
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(b), nil
}
