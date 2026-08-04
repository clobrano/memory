package db

import (
	"database/sql"
	"errors"
	"path/filepath"
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

// A card scheduled for tomorrow must not be offered today: the scheduler's
// minimum interval is one day, and a note may never come back on the day it
// was reviewed.
func TestCardScheduledForTomorrowIsNotDueToday(t *testing.T) {
	database := openTestDBFile(t)

	c := Card{
		Path: "/notes/tomorrow.md", Title: "Tomorrow", Tag: "#study",
		FirstIndexed: time.Now(), LastReview: time.Now(),
		NextDue: time.Now().AddDate(0, 0, 1), Reps: 1, State: "review",
	}
	if _, err := UpsertCard(database, c); err != nil {
		t.Fatalf("UpsertCard: %v", err)
	}

	cards, err := GetDueCards(database, nil)
	if err != nil {
		t.Fatalf("GetDueCards: %v", err)
	}
	if len(cards) != 0 {
		t.Errorf("got %d due cards, want 0 — a card due tomorrow was offered today", len(cards))
	}
}

// The card falls due today at some clock time. Stored as a full timestamp it
// sorted after today's date and stayed hidden until tomorrow, adding a day to
// every interval in the app.
func TestCardDueTodayIsOfferedToday(t *testing.T) {
	database := openTestDBFile(t)

	c := Card{
		Path: "/notes/today.md", Title: "Today", Tag: "#study",
		FirstIndexed: time.Now().AddDate(0, 0, -1),
		LastReview:   time.Now().AddDate(0, 0, -1),
		NextDue:      time.Now(), Reps: 1, State: "review",
	}
	if _, err := UpsertCard(database, c); err != nil {
		t.Fatalf("UpsertCard: %v", err)
	}

	cards, err := GetDueCards(database, nil)
	if err != nil {
		t.Fatalf("GetDueCards: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("got %d due cards, want 1 — a card due today was not offered", len(cards))
	}
}

func TestNextDueStoredAsDayOnly(t *testing.T) {
	database := openTestDBFile(t)

	id, err := UpsertCard(database, Card{
		Path: "/notes/day.md", Title: "Day", Tag: "#study", FirstIndexed: time.Now(),
	})
	if err != nil {
		t.Fatalf("UpsertCard: %v", err)
	}

	// A new note carries no due date at all, and must be stored as unscheduled
	// rather than as the zero time formatted into the column.
	assertStoredNextDue(t, database, "/notes/day.md", "")

	card, err := GetCardByID(database, id)
	if err != nil {
		t.Fatalf("GetCardByID: %v", err)
	}
	card.NextDue = time.Now().AddDate(0, 0, 6)
	card.Reps, card.State = 2, "review"
	if err := UpdateCardSchedule(database, *card); err != nil {
		t.Fatalf("UpdateCardSchedule: %v", err)
	}

	want := time.Now().AddDate(0, 0, 6).Format("2006-01-02")
	assertStoredNextDue(t, database, "/notes/day.md", want)

	round, err := GetCardByID(database, id)
	if err != nil {
		t.Fatalf("GetCardByID after schedule: %v", err)
	}
	if got := round.NextDue.Format("2006-01-02"); got != want {
		t.Errorf("NextDue read back as %q, want %q", got, want)
	}
}

// Sessions are capped at the daily limit, so cards already in circulation have
// to come first — otherwise a vault full of unread notes starves the reviews
// that would graduate to a longer interval.
func TestGetDueCardsPrioritizesReviewsOverNewNotes(t *testing.T) {
	database := openTestDBFile(t)

	now := time.Now()
	for _, c := range []Card{
		{Path: "/notes/new-older.md", Title: "New older", Tag: "#study",
			FirstIndexed: now.AddDate(0, 0, -3)},
		{Path: "/notes/new-newer.md", Title: "New newer", Tag: "#study",
			FirstIndexed: now.AddDate(0, 0, -1)},
		// Lapsed: the scheduler resets reps to 0, but the card has been seen.
		{Path: "/notes/lapsed.md", Title: "Lapsed", Tag: "#study",
			FirstIndexed: now.AddDate(0, 0, -10), LastReview: now.AddDate(0, 0, -1),
			NextDue: now, Reps: 0, Lapses: 1, State: "learning"},
		{Path: "/notes/overdue.md", Title: "Overdue", Tag: "#study",
			FirstIndexed: now.AddDate(0, 0, -20), LastReview: now.AddDate(0, 0, -8),
			NextDue: now.AddDate(0, 0, -5), Reps: 3, State: "review"},
	} {
		if _, err := UpsertCard(database, c); err != nil {
			t.Fatalf("UpsertCard %s: %v", c.Path, err)
		}
	}

	cards, err := GetDueCards(database, nil)
	if err != nil {
		t.Fatalf("GetDueCards: %v", err)
	}

	var got []string
	for _, c := range cards {
		got = append(got, c.Path)
	}
	want := []string{
		"/notes/overdue.md",   // seen, most overdue
		"/notes/lapsed.md",    // seen, due today
		"/notes/new-newer.md", // unseen, newest first
		"/notes/new-older.md",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d = %s, want %s (full order %v)", i, got[i], want[i], got)
		}
	}
}

func TestMigrationNormalizesLegacyNextDue(t *testing.T) {
	database := openTestDBFile(t)

	// Rows as written before day-only storage: Go's time.Time.String() format
	// for a card due today, and the zero time for a card never scheduled.
	insert := `INSERT INTO cards(path,title,tag,first_indexed,stability,difficulty,
		elapsed_days,scheduled_days,reps,lapses,state,last_review,next_due)
		VALUES(?,?,'#study','2026-07-01 09:00:00.1 +0200 CEST',2.15,2,1,1,?,1,?,'',?)`
	today := time.Now().Format("2006-01-02")
	if _, err := database.Exec(insert, "/notes/legacy.md", "Legacy", 1, "review",
		today+" 11:45:41.976585154 +0200 CEST"); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if _, err := database.Exec(insert, "/notes/unscheduled.md", "Unscheduled", 0, "",
		"0001-01-01 00:00:00 +0000 UTC"); err != nil {
		t.Fatalf("insert zero-time row: %v", err)
	}

	// Replay the normalising migrations over the rows written above.
	if _, err := database.Exec(
		`DELETE FROM migrations WHERE name IN ('006_next_due_zero_to_empty','007_next_due_day_only')`); err != nil {
		t.Fatalf("reset migration records: %v", err)
	}
	if err := RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	assertStoredNextDue(t, database, "/notes/legacy.md", today)
	assertStoredNextDue(t, database, "/notes/unscheduled.md", "")

	// Both are due now: the legacy card falls due today, the unscheduled one
	// has never been seen.
	cards, err := GetDueCards(database, nil)
	if err != nil {
		t.Fatalf("GetDueCards: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d due cards, want 2", len(cards))
	}
	if cards[0].Path != "/notes/legacy.md" {
		t.Errorf("first due card = %s, want the seen card /notes/legacy.md", cards[0].Path)
	}
}

// assertStoredNextDue checks the text actually held in the column. The driver
// re-parses columns declared DATETIME into time.Time, which database/sql then
// renders as RFC3339 when scanned into a string, so a Go-side comparison would
// describe the read path rather than the stored value. Comparing and measuring
// inside SQLite sidesteps that: length is the decisive part, since a day-only
// value is 10 characters where any timestamp form is longer.
func assertStoredNextDue(t *testing.T, db *sql.DB, path, want string) {
	t.Helper()
	var matches bool
	var length int
	var rendered string
	if err := db.QueryRow(`SELECT next_due = ?, length(next_due), CAST(next_due AS TEXT)
		FROM cards WHERE path=?`, want, path).Scan(&matches, &length, &rendered); err != nil {
		t.Fatalf("read next_due for %s: %v", path, err)
	}
	if !matches || length != len(want) {
		t.Errorf("stored next_due for %s = %q (length %d), want %q (length %d)",
			path, rendered, length, want, len(want))
	}
}

func openTestDBFile(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := RunMigrations(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
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
