package db

import (
	"database/sql"
	"time"
)

type Review struct {
	ID         int64
	CardID     int64
	ReviewedAt time.Time
	Grade      string
	Rating     int
}

func InsertReview(db *sql.DB, r Review) error {
	_, err := db.Exec(`INSERT INTO review_history(card_id,reviewed_at,grade,rating) VALUES(?,?,?,?)`,
		r.CardID, r.ReviewedAt, r.Grade, r.Rating)
	return err
}

func GetReviewsLast30Days(db *sql.DB) ([]Review, error) {
	since := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	rows, err := db.Query(`SELECT id,card_id,reviewed_at,grade,rating FROM review_history WHERE reviewed_at >= ? ORDER BY reviewed_at`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReviews(rows)
}

func GetReviewsPerDay(db *sql.DB, days int) (map[string]int, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := db.Query(`SELECT date(reviewed_at), COUNT(*) FROM review_history WHERE reviewed_at >= ? GROUP BY date(reviewed_at)`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var day string
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		result[day] = count
	}
	return result, rows.Err()
}

func scanReviews(rows *sql.Rows) ([]Review, error) {
	var reviews []Review
	for rows.Next() {
		var r Review
		var reviewedAt string
		if err := rows.Scan(&r.ID, &r.CardID, &reviewedAt, &r.Grade, &r.Rating); err != nil {
			return nil, err
		}
		if t, err := time.Parse("2006-01-02 15:04:05", reviewedAt); err == nil {
			r.ReviewedAt = t
		} else if t, err := time.Parse(time.RFC3339, reviewedAt); err == nil {
			r.ReviewedAt = t
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}
