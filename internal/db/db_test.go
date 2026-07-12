package db

import (
	"database/sql"
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
		Path:  "/notes/foo.md",
		Title: "Foo",
		Tag:   "#study",
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
