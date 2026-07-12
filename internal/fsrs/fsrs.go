package fsrs

import (
	"fmt"
	"time"

	"github.com/clobrano/memory/internal/db"
)

type Grade int

const (
	GradeAllCorrect       Grade = 3
	GradePartiallyCorrect Grade = 2
	GradeNeedsReview      Grade = 1
)

func (g Grade) String() string {
	switch g {
	case GradeAllCorrect:
		return "All correct"
	case GradePartiallyCorrect:
		return "Partially correct"
	case GradeNeedsReview:
		return "Needs review"
	default:
		return "Unknown"
	}
}

func (g Grade) Rating() int {
	switch g {
	case GradeAllCorrect:
		return 3
	case GradePartiallyCorrect:
		return 2
	default:
		return 1
	}
}

const defaultEaseFactor = 2.5

// Schedule applies SM-2 and returns an updated card.
// The card's Stability field stores the ease factor; Difficulty stores the interval in days.
func Schedule(card db.Card, grade Grade, now time.Time) (db.Card, error) {
	easeFactor := card.Stability
	if easeFactor < 1.3 {
		easeFactor = defaultEaseFactor
	}
	interval := card.ScheduledDays
	reps := card.Reps
	lapses := card.Lapses

	var nextInterval int
	var newState string

	switch grade {
	case GradeAllCorrect:
		if reps == 0 {
			nextInterval = 1
		} else if reps == 1 {
			nextInterval = 6
		} else {
			nextInterval = int(float64(interval) * easeFactor)
		}
		easeFactor += 0.1
		reps++
		newState = "review"
	case GradePartiallyCorrect:
		if reps == 0 {
			nextInterval = 1
		} else if reps == 1 {
			nextInterval = 4
		} else {
			nextInterval = int(float64(interval) * easeFactor)
		}
		easeFactor -= 0.15
		reps++
		newState = "review"
	case GradeNeedsReview:
		nextInterval = 1
		easeFactor -= 0.2
		lapses++
		reps = 0
		newState = "learning"
	default:
		return card, fmt.Errorf("unknown grade: %d", grade)
	}

	if easeFactor < 1.3 {
		easeFactor = 1.3
	}
	if nextInterval < 1 {
		nextInterval = 1
	}

	elapsed := 0
	if !card.LastReview.IsZero() {
		elapsed = int(now.Sub(card.LastReview).Hours() / 24)
	}

	card.Stability = easeFactor
	card.Difficulty = float64(grade.Rating())
	card.ElapsedDays = elapsed
	card.ScheduledDays = nextInterval
	card.Reps = reps
	card.Lapses = lapses
	card.State = newState
	card.LastReview = now
	card.NextDue = now.AddDate(0, 0, nextInterval)
	return card, nil
}
