package db

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrationsIdempotent(t *testing.T) {
	db := openTestDB(t)
	// Running a second time must not error
	if err := RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}
}

func TestUpsertCard(t *testing.T) {
	db := openTestDB(t)
	c := Card{
		Path:         "/notes/foo.md",
		Title:        "Foo",
		Tag:          "#study",
		FirstIndexed: time.Now(),
	}
	id, err := UpsertCard(db, c)
	if err != nil {
		t.Fatalf("UpsertCard: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero id on insert")
	}

	// Upsert again — should update title, not duplicate
	c.Title = "Foo Updated"
	_, err = UpsertCard(db, c)
	if err != nil {
		t.Fatalf("UpsertCard (update): %v", err)
	}

	got, err := GetCardByPath(db, "/notes/foo.md")
	if err != nil {
		t.Fatalf("GetCardByPath: %v", err)
	}
	if got.Title != "Foo Updated" {
		t.Errorf("Title = %q, want 'Foo Updated'", got.Title)
	}
}

func TestGetDueCardsFiltering(t *testing.T) {
	db := openTestDB(t)

	yesterday := time.Now().AddDate(0, 0, -1)
	tomorrow := time.Now().AddDate(0, 0, 1)

	due := Card{Path: "/notes/due.md", Title: "Due", Tag: "#study", FirstIndexed: time.Now(), NextDue: yesterday}
	notDue := Card{Path: "/notes/future.md", Title: "Future", Tag: "#study", FirstIndexed: time.Now(), NextDue: tomorrow}

	if _, err := UpsertCard(db, due); err != nil {
		t.Fatal(err)
	}
	if _, err := UpsertCard(db, notDue); err != nil {
		t.Fatal(err)
	}

	cards, err := GetDueCards(db, nil)
	if err != nil {
		t.Fatalf("GetDueCards: %v", err)
	}
	if len(cards) != 1 {
		t.Errorf("got %d due cards, want 1", len(cards))
	}
	if len(cards) > 0 && cards[0].Path != "/notes/due.md" {
		t.Errorf("due card path = %s, want /notes/due.md", cards[0].Path)
	}
}

func TestCachedQuestionsRoundTrip(t *testing.T) {
	db := openTestDB(t)

	id, err := UpsertCard(db, Card{
		Path:         "/notes/cache.md",
		Title:        "Cache",
		Tag:          "#study",
		FirstIndexed: time.Now(),
	})
	if err != nil {
		t.Fatalf("UpsertCard: %v", err)
	}

	// The cache columns are added by migration without a default, so a card that
	// has never been cached holds NULL in all three.
	got, err := GetCardByID(db, id)
	if err != nil {
		t.Fatalf("GetCardByID: %v", err)
	}
	if got.NoteContentHash != "" || got.CachedQuestions != "" || got.CachedQuestionsTimestamp != 0 {
		t.Errorf("uncached card = (%q, %q, %d), want zero values",
			got.NoteContentHash, got.CachedQuestions, got.CachedQuestionsTimestamp)
	}

	if err := UpdateCachedQuestions(db, id, "deadbeef", "Q1\n---\nS1"); err != nil {
		t.Fatalf("UpdateCachedQuestions: %v", err)
	}

	got, err = GetCardByID(db, id)
	if err != nil {
		t.Fatalf("GetCardByID after cache write: %v", err)
	}
	if got.NoteContentHash != "deadbeef" {
		t.Errorf("NoteContentHash = %q, want %q", got.NoteContentHash, "deadbeef")
	}
	if got.CachedQuestions != "Q1\n---\nS1" {
		t.Errorf("CachedQuestions = %q, want %q", got.CachedQuestions, "Q1\n---\nS1")
	}
	if got.CachedQuestionsTimestamp == 0 {
		t.Error("CachedQuestionsTimestamp = 0, want the write time")
	}
}

func TestMergeCardHistories(t *testing.T) {
	db := openTestDB(t)

	older := time.Date(2020, 1, 15, 12, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	// The surviving card was indexed later and already has some history.
	survivorID, err := UpsertCard(db, Card{
		Path: "/notes/survivor.md", Title: "Survivor", Tag: "#study",
		FirstIndexed: newer, Reps: 3, Lapses: 1,
	})
	if err != nil {
		t.Fatalf("UpsertCard survivor: %v", err)
	}

	// The stale card carries the older first_indexed date and more history.
	staleID, err := UpsertCard(db, Card{
		Path: "/notes/stale.md", Title: "Stale", Tag: "#study",
		FirstIndexed: older, Reps: 5, Lapses: 2,
	})
	if err != nil {
		t.Fatalf("UpsertCard stale: %v", err)
	}

	for i := 0; i < 2; i++ {
		if err := InsertReview(db, Review{CardID: staleID, ReviewedAt: time.Now(), Grade: "all-correct", Rating: 3}); err != nil {
			t.Fatalf("InsertReview stale: %v", err)
		}
	}
	if err := InsertReview(db, Review{CardID: survivorID, ReviewedAt: time.Now(), Grade: "needs-review", Rating: 1}); err != nil {
		t.Fatalf("InsertReview survivor: %v", err)
	}

	// Compare first_indexed as stored, so the assertion does not depend on how
	// the driver formats DATETIME values.
	wantFirstIndexed := rawFirstIndexed(t, db, staleID)

	stale, err := GetCardByID(db, staleID)
	if err != nil {
		t.Fatalf("GetCardByID stale: %v", err)
	}
	survivor, err := GetCardByID(db, survivorID)
	if err != nil {
		t.Fatalf("GetCardByID survivor: %v", err)
	}

	if err := MergeCardHistories(db, *stale, *survivor); err != nil {
		t.Fatalf("MergeCardHistories: %v", err)
	}

	merged, err := GetCardByID(db, survivorID)
	if err != nil {
		t.Fatalf("GetCardByID merged: %v", err)
	}
	if merged.Reps != 8 {
		t.Errorf("Reps = %d, want 8 (3+5)", merged.Reps)
	}
	if merged.Lapses != 3 {
		t.Errorf("Lapses = %d, want 3 (1+2)", merged.Lapses)
	}
	if got := rawFirstIndexed(t, db, survivorID); got != wantFirstIndexed {
		t.Errorf("first_indexed = %q, want the older date %q", got, wantFirstIndexed)
	}

	if n := countReviews(t, db, survivorID); n != 3 {
		t.Errorf("survivor has %d reviews, want 3 (1 own + 2 moved)", n)
	}
	if n := countReviews(t, db, staleID); n != 0 {
		t.Errorf("stale card still has %d reviews, want 0", n)
	}

	if _, err := GetCardByID(db, staleID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetCardByID(stale) error = %v, want sql.ErrNoRows", err)
	}
}

func rawFirstIndexed(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	var v sql.NullString
	if err := db.QueryRow(`SELECT first_indexed FROM cards WHERE id=?`, id).Scan(&v); err != nil {
		t.Fatalf("read first_indexed for card %d: %v", id, err)
	}
	return v.String
}

func countReviews(t *testing.T, db *sql.DB, cardID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM review_history WHERE card_id=?`, cardID).Scan(&n); err != nil {
		t.Fatalf("count reviews for card %d: %v", cardID, err)
	}
	return n
}
