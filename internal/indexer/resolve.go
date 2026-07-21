package indexer

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/clobrano/memory/internal/db"
)

func ResolveMissing(database *sql.DB, missing []db.Card) error {
	if len(missing) == 0 {
		return nil
	}
	reader := bufio.NewReader(os.Stdin)
	for _, card := range missing {
		fmt.Printf("\nMissing note: %s\n", card.Path)
		fmt.Print("[r] remap path  [d] delete history  [s] skip: ")
		line, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(strings.ToLower(line))
		switch choice {
		case "r":
			fmt.Print("New path: ")
			newPath, _ := reader.ReadString('\n')
			newPath = strings.TrimSpace(newPath)
			if err := remapWithConflictHandling(database, reader, card, newPath); err != nil {
				fmt.Fprintf(os.Stderr, "remap error: %v\n", err)
			}
		case "d":
			if err := db.DeleteCard(database, card.ID); err != nil {
				fmt.Fprintf(os.Stderr, "delete error: %v\n", err)
			}
		default:
			// skip
		}
	}
	return nil
}

func remapWithConflictHandling(database *sql.DB, reader *bufio.Reader, oldCard db.Card, newPath string) error {
	// Check if the new path already exists in the database
	existingCard, err := db.GetCardByPath(database, newPath)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("check existing path: %w", err)
	}

	// If no conflict, just update the path
	if existingCard == nil {
		if err := db.UpdateCardPath(database, oldCard.ID, newPath); err != nil {
			return fmt.Errorf("update path: %w", err)
		}
		return nil
	}

	// Conflict detected: new path already exists
	fmt.Printf("\nConflict: The new path '%s' already exists in your records.\n", newPath)
	fmt.Printf("  Existing: %s (ID: %d, %d reviews)\n", existingCard.Path, existingCard.ID, existingCard.Reps)
	fmt.Printf("  Missing:  %s (ID: %d, %d reviews)\n", oldCard.Path, oldCard.ID, oldCard.Reps)
	fmt.Print("\nThese likely refer to the same study material. Merge the histories?\n")
	fmt.Print("[y] merge (keep older history)  [n] cancel: ")
	line, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(line))

	if choice != "y" {
		return nil
	}

	// Merge the histories
	if err := mergeCardHistories(database, oldCard, *existingCard); err != nil {
		return fmt.Errorf("merge histories: %w", err)
	}

	fmt.Printf("Histories merged. The study material at '%s' now has combined review history.\n", newPath)
	return nil
}

func mergeCardHistories(database *sql.DB, oldCard, newCard db.Card) error {
	// Move all reviews from oldCard to newCard
	if err := db.UpdateReviewCardID(database, oldCard.ID, newCard.ID); err != nil {
		return fmt.Errorf("move reviews: %w", err)
	}

	// Merge card data (combine reps, lapses, keep older first_indexed)
	if err := db.MergeCards(database, &oldCard, &newCard); err != nil {
		return fmt.Errorf("merge card data: %w", err)
	}

	// Delete the old card record
	if err := db.DeleteCard(database, oldCard.ID); err != nil {
		return fmt.Errorf("delete old card: %w", err)
	}

	return nil
}
