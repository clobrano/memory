package db

import (
	"database/sql"
	"time"
)

// ComputeStreak counts consecutive days of reviews up to and including today.
func ComputeStreak(db *sql.DB) (int, error) {
	rows, err := db.Query(`SELECT DISTINCT date(reviewed_at) FROM review_history WHERE reviewed_at IS NOT NULL ORDER BY date(reviewed_at) DESC`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	streak := 0
	expected := time.Now().Truncate(24 * time.Hour)
	for rows.Next() {
		var day sql.NullString
		if err := rows.Scan(&day); err != nil {
			return streak, err
		}
		if !day.Valid {
			continue
		}
		t, err := time.Parse("2006-01-02", day.String)
		if err != nil {
			continue
		}
		if t.Equal(expected) || t.Equal(expected.AddDate(0, 0, 1)) {
			// allow today or yesterday to start streak
			streak++
			expected = t.AddDate(0, 0, -1)
		} else {
			break
		}
	}
	return streak, rows.Err()
}
