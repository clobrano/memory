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
	inFrontmatter := false
	frontmatterCount := 0
	contentStart := 0

	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			frontmatterCount++
			if frontmatterCount == 2 {
				// Found end of frontmatter, start from next line
				contentStart = i + 1
				break
			} else if frontmatterCount == 1 {
				inFrontmatter = true
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

	// If no title found, start from content start
	if titleStart == -1 {
		titleStart = contentStart
	}

	// Skip tags after title (lines that start with # but aren't headings)
	contentLinesStart := titleStart
	if titleStart < len(lines) {
		// Skip the title line itself
		contentLinesStart = titleStart + 1

		// Skip tag lines (lines that are just tags like "#tag1 #tag2")
		for i := contentLinesStart; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" {
				// Skip blank lines
				continue
			}
			// Check if line is all tags (starts with #, isn't a heading)
			if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "# ") {
				// It's a tag line, skip it
				continue
			}
			// Found first non-tag line
			contentLinesStart = i
			break
		}
	}

	// Rebuild content from title onwards
	if titleStart >= 0 && titleStart < len(lines) {
		result := strings.Join(lines[titleStart:], "\n")
		return strings.TrimSpace(result)
	}

	return strings.TrimSpace(strings.Join(lines[contentStart:], "\n"))
}

// ComputeNoteHash returns SHA256 hex of extracted content
func ComputeNoteHash(fullText string) string {
	content := ExtractNoteContent(fullText)
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}
