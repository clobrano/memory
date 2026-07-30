package db

import (
	"database/sql"
	"fmt"
	"time"
)

type migration struct {
	name string
	sql  string
}

var migrations = []migration{
	{
		name: "001_cards",
		sql: `CREATE TABLE IF NOT EXISTS cards (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			path          TEXT UNIQUE NOT NULL,
			title         TEXT,
			tag           TEXT,
			first_indexed DATETIME,
			stability     REAL,
			difficulty    REAL,
			elapsed_days  INTEGER,
			scheduled_days INTEGER,
			reps          INTEGER,
			lapses        INTEGER,
			state         TEXT,
			last_review   DATETIME,
			next_due      DATETIME
		)`,
	},
	{
		name: "002_reviews",
		sql: `CREATE TABLE IF NOT EXISTS review_history (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			card_id     INTEGER REFERENCES cards(id),
			reviewed_at DATETIME,
			grade       TEXT,
			rating      INTEGER
		)`,
	},
	{
		name: "003_cache_questions",
		sql:  `ALTER TABLE cards ADD COLUMN note_content_hash TEXT`,
	},
	{
		name: "004_cache_questions_content",
		sql:  `ALTER TABLE cards ADD COLUMN cached_questions TEXT`,
	},
	{
		name: "005_cache_questions_timestamp",
		sql:  `ALTER TABLE cards ADD COLUMN cached_questions_timestamp INTEGER`,
	},
	// 006 and 007 bring next_due onto the day-only form GetDueCards compares
	// against. Rows written before this were full timestamps, which sort after
	// the bare date they fall on, so every card stayed hidden for a day longer
	// than it was scheduled for.
	{
		name: "006_next_due_zero_to_empty",
		sql:  `UPDATE cards SET next_due = '' WHERE next_due LIKE '0001-01-01%'`,
	},
	{
		name: "007_next_due_day_only",
		sql: `UPDATE cards SET next_due = substr(next_due, 1, 10)
			WHERE next_due IS NOT NULL AND length(next_due) > 10`,
	},
}

func RunMigrations(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT UNIQUE,
		applied_at DATETIME
	)`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	for _, m := range migrations {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM migrations WHERE name=?)`, m.name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", m.name, err)
		}
		if exists {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		if _, err := db.Exec(`INSERT INTO migrations(name,applied_at) VALUES(?,?)`, m.name, time.Now()); err != nil {
			return fmt.Errorf("record migration %s: %w", m.name, err)
		}
	}
	return nil
}
