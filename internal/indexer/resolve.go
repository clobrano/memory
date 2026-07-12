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
			if err := db.UpdateCardPath(database, card.ID, newPath); err != nil {
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
