package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/clobrano/memory/internal/db"
)

var (
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

var listCmd = &cobra.Command{
	Use:   "list [keywords...]",
	Short: "List all tracked notes",
	RunE: func(cmd *cobra.Command, args []string) error {
		cards, err := db.ListAllCards(DB)
		if err != nil {
			return err
		}

		// keyword filter
		if len(args) > 0 {
			var filtered []db.Card
			for _, c := range cards {
				data, err := os.ReadFile(c.Path)
				if err != nil {
					continue
				}
				lower := strings.ToLower(string(data))
				match := true
				for _, kw := range args {
					if !strings.Contains(lower, strings.ToLower(kw)) {
						match = false
						break
					}
				}
				if match {
					filtered = append(filtered, c)
				}
			}
			cards = filtered
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TITLE\tTAG\tNEXT DUE\tSTATE")
		fmt.Fprintln(w, "-----\t---\t--------\t-----")

		now := time.Now().Truncate(24 * time.Hour)
		for _, c := range cards {
			due := "never"
			rowColor := ""
			if !c.NextDue.IsZero() {
				due = c.NextDue.Format("2006-01-02")
				d := c.NextDue.Truncate(24 * time.Hour)
				if d.Before(now) {
					rowColor = "red"
				} else if d.Equal(now) {
					rowColor = "yellow"
				}
			}

			line := fmt.Sprintf("%s\t%s\t%s\t%s", c.Title, c.Tag, due, c.State)
			switch rowColor {
			case "red":
				fmt.Fprintln(w, redStyle.Render(line))
			case "yellow":
				fmt.Fprintln(w, yellowStyle.Render(line))
			default:
				fmt.Fprintln(w, line)
			}
		}
		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
