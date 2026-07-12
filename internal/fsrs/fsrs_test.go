package fsrs

import (
	"testing"
	"time"

	"github.com/clobrano/memory/internal/db"
)

func newCard() db.Card {
	return db.Card{
		ID:    1,
		Path:  "/notes/test.md",
		Title: "Test",
	}
}

func TestAgainSchedulesNextDay(t *testing.T) {
	card := newCard()
	now := time.Now()

	updated, err := Schedule(card, GradeNeedsReview, now)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	due := updated.NextDue.Truncate(24 * time.Hour)
	want := now.AddDate(0, 0, 1).Truncate(24 * time.Hour)
	if !due.Equal(want) {
		t.Errorf("NextDue = %v, want %v", due, want)
	}
	if updated.State != "learning" {
		t.Errorf("State = %q, want 'learning'", updated.State)
	}
}

func TestGoodGradesIncreaseInterval(t *testing.T) {
	card := newCard()
	now := time.Now()

	for i := 0; i < 5; i++ {
		var err error
		card, err = Schedule(card, GradeAllCorrect, now)
		if err != nil {
			t.Fatal(err)
		}
		now = card.NextDue
	}

	if card.ScheduledDays <= 6 {
		t.Errorf("expected interval > 6 after multiple Good grades, got %d", card.ScheduledDays)
	}
}

func TestEaseFactorFloorsAt1_3(t *testing.T) {
	card := newCard()
	now := time.Now()

	for i := 0; i < 10; i++ {
		var err error
		card, err = Schedule(card, GradeNeedsReview, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	if card.Stability < 1.3 {
		t.Errorf("ease factor = %.2f, want >= 1.3", card.Stability)
	}
}
