package cache

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ExtractNoteContent removes YAML frontmatter and tags, returning only title + body
func ExtractNoteContent(fullText string) string {
	lines := strings.Split(fullText, "\n")
	contentStart := skipFrontmatter(lines)

	titleIdx := -1
	for i := contentStart; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "# ") {
			titleIdx = i
			break
		}
	}

	if titleIdx == -1 {
		return strings.TrimSpace(strings.Join(lines[contentStart:], "\n"))
	}

	// Body begins at the first line after the title that is neither blank nor a tag line.
	bodyStart := len(lines)
	for i := titleIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || isTagLine(trimmed) {
			continue
		}
		bodyStart = i
		break
	}

	title := strings.TrimSpace(lines[titleIdx])
	body := strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	if body == "" {
		return title
	}
	return title + "\n\n" + body
}

// skipFrontmatter returns the index of the first line after a leading YAML
// frontmatter block, or 0 when the text does not open with one.
func skipFrontmatter(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i + 1
		}
	}
	return 0
}

// isTagLine reports whether every token on the line is a "#tag", which
// distinguishes a tag line from a Markdown heading such as "## Section".
func isTagLine(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	for _, f := range fields {
		if len(f) < 2 || f[0] != '#' || f[1] == '#' {
			return false
		}
	}
	return true
}

// ComputeNoteHash returns SHA256 hex of extracted content
func ComputeNoteHash(fullText string) string {
	content := ExtractNoteContent(fullText)
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}
