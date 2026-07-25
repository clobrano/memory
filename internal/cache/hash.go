package cache

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ExtractNoteContent removes YAML frontmatter and tags, returning only title + body
func ExtractNoteContent(fullText string) string {
	lines := strings.Split(fullText, "\n")

	// Skip YAML frontmatter
	frontmatterCount := 0
	contentStart := 0

	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			frontmatterCount++
			if frontmatterCount == 2 {
				// Found end of frontmatter, start from next line
				contentStart = i + 1
				break
			}
		}
	}

	// Skip blank lines after frontmatter
	for i := contentStart; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			contentStart = i
			break
		}
	}

	// Find the title (first line with "# ")
	titleStart := -1
	for i := contentStart; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "# ") {
			titleStart = i
			break
		}
	}

	// If no title found, return from content start
	if titleStart == -1 {
		return strings.TrimSpace(strings.Join(lines[contentStart:], "\n"))
	}

	// Find where actual body content starts (skip title and all tag lines)
	bodyStart := titleStart + 1

	for i := bodyStart; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		// Skip blank lines
		if trimmed == "" {
			continue
		}

		// Check if this line is a tag line
		// Tags are lines that: start with # but NOT with "# " (which is heading)
		// AND contain only tags (words starting with #)
		if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "# ") {
			// This looks like a tag line, skip it
			continue
		}

		// Found first non-tag, non-blank line - this is where body starts
		bodyStart = i
		break
	}

	// Rebuild content from title onwards (including title and body, but not tags)
	result := make([]string, 0)
	result = append(result, lines[titleStart])

	// Add blank line after title if there is body content
	if bodyStart < len(lines) && strings.TrimSpace(lines[bodyStart]) != "" {
		result = append(result, "")
	}

	// Add all remaining lines as body
	result = append(result, lines[bodyStart:]...)

	return strings.TrimSpace(strings.Join(result, "\n"))
}

// ComputeNoteHash returns SHA256 hex of extracted content
func ComputeNoteHash(fullText string) string {
	content := ExtractNoteContent(fullText)
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}
