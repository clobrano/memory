package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/clobrano/memory/internal/db"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show study statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Streak
		streak, err := db.ComputeStreak(DB)
		if err != nil {
			return err
		}

		// Totals
		allCards, err := db.ListAllCards(DB)
		if err != nil {
			return err
		}
		now := time.Now()
		today := now.Truncate(24 * time.Hour)
		var dueToday, overdue int
		for _, c := range allCards {
			if c.NextDue.IsZero() {
				continue
			}
			d := c.NextDue.Truncate(24 * time.Hour)
			if d.Before(today) {
				overdue++
				dueToday++
			} else if d.Equal(today) {
				dueToday++
			}
		}

		// 30-day retention
		reviews, err := db.GetReviewsLast30Days(DB)
		if err != nil {
			return err
		}
		var correct, total int
		for _, r := range reviews {
			total++
			if r.Grade == "All correct" {
				correct++
			}
		}
		var retention float64
		if total > 0 {
			retention = float64(correct) / float64(total) * 100
		}

		// Reviews per day (14 days)
		perDay, err := db.GetReviewsPerDay(DB, 14)
		if err != nil {
			return err
		}

		fmt.Println("=== Memory Stats ===")
		fmt.Printf("\nStreak:       %d days\n", streak)
		fmt.Printf("Total cards:  %d\n", len(allCards))
		fmt.Printf("Due today:    %d\n", dueToday)
		fmt.Printf("Overdue:      %d\n", overdue)
		fmt.Printf("30d retention: %.1f%%  (%d/%d correct)\n\n", retention, correct, total)

		fmt.Println("--- Reviews per day (last 14 days) ---")
		var maxCount int
		for _, c := range perDay {
			if c > maxCount {
				maxCount = c
			}
		}
		const barWidth = 40
		for i := 13; i >= 0; i-- {
			day := now.AddDate(0, 0, -i).Format("2006-01-02")
			count := perDay[day]
			barLen := 0
			if maxCount > 0 {
				barLen = count * barWidth / maxCount
			}
			bar := strings.Repeat("█", barLen)
			fmt.Printf("%s  %-40s %d\n", day[5:], bar, count)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
