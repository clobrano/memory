package cache

import (
	"strings"
	"testing"
)

func TestExtractNoteContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "full note with frontmatter tags and body",
			input: `---
created: 2024-01-01
modified: 2024-01-02
---

# My Title
#tag1 #tag2

Body line 1
Body line 2`,
			expected: `# My Title

Body line 1
Body line 2`,
		},
		{
			name: "no frontmatter",
			input: `# Title
#tag

Content here`,
			expected: `# Title

Content here`,
		},
		{
			name: "multiple tag lines",
			input: `---
created: 2024-01-01
---

# Title
#tag1 #tag2
#tag3

Content`,
			expected: `# Title

Content`,
		},
		{
			name: "tags on separate lines",
			input: `---
created: 2024-01-01
---

# Title
#tag1
#tag2

First line
Second line`,
			expected: `# Title

First line
Second line`,
		},
		{
			name: "blank lines between title and tags",
			input: `---
created: 2024-01-01
---

# Title

#tag1 #tag2

Content`,
			expected: `# Title

Content`,
		},
		{
			name: "no tags after title",
			input: `---
created: 2024-01-01
---

# Title

Content line 1
Content line 2`,
			expected: `# Title

Content line 1
Content line 2`,
		},
		{
			name: "minimal note",
			input: `# Title

Content`,
			expected: `# Title

Content`,
		},
		{
			name: "content with hashtags in text",
			input: `---
created: 2024-01-01
---

# Title
#tag

This is content about #python and #golang`,
			expected: `# Title

This is content about #python and #golang`,
		},
		{
			name: "modified date in frontmatter only (should be ignored)",
			input: `---
created: 2024-01-01
modified: 2024-01-15
---

# Title
#tag

Content`,
			expected: `# Title

Content`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractNoteContent(tt.input)
			if strings.TrimSpace(result) != strings.TrimSpace(tt.expected) {
				t.Errorf("ExtractNoteContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestComputeNoteHash(t *testing.T) {
	// Test 1: Same content produces same hash (deterministic)
	content := `---
created: 2024-01-01
modified: 2024-01-02
---

# My Title
#tag1 #tag2

Body content here`

	hash1 := ComputeNoteHash(content)
	hash2 := ComputeNoteHash(content)

	if hash1 != hash2 {
		t.Errorf("ComputeNoteHash() not deterministic: got %q and %q", hash1, hash2)
	}

	// Test 2: Different content produces different hash
	contentModified := `---
created: 2024-01-01
modified: 2024-01-02
---

# My Title
#tag1 #tag2

Body content CHANGED`

	hash3 := ComputeNoteHash(contentModified)
	if hash1 == hash3 {
		t.Errorf("ComputeNoteHash() should differ for different content")
	}

	// Test 3: Modified date doesn't change hash (only content matters)
	contentDateChanged := `---
created: 2024-01-01
modified: 2024-02-15
---

# My Title
#tag1 #tag2

Body content here`

	hash4 := ComputeNoteHash(contentDateChanged)
	if hash1 != hash4 {
		t.Errorf("ComputeNoteHash() should ignore metadata changes (dates)")
	}

	// Test 4: Tag changes don't affect hash
	contentTagChanged := `---
created: 2024-01-01
modified: 2024-01-02
---

# My Title
#tag1 #tag2 #tag3

Body content here`

	hash5 := ComputeNoteHash(contentTagChanged)
	if hash1 != hash5 {
		t.Errorf("ComputeNoteHash() should ignore tag changes")
	}

	// Test 5: Title changes DO affect hash
	contentTitleChanged := `---
created: 2024-01-01
modified: 2024-01-02
---

# My Title UPDATED
#tag1 #tag2

Body content here`

	hash6 := ComputeNoteHash(contentTitleChanged)
	if hash1 == hash6 {
		t.Errorf("ComputeNoteHash() should detect title changes")
	}

	// Test 6: Body whitespace matters
	contentWhitespaceChanged := `---
created: 2024-01-01
modified: 2024-01-02
---

# My Title
#tag1 #tag2

Body  content  here`

	hash7 := ComputeNoteHash(contentWhitespaceChanged)
	if hash1 == hash7 {
		t.Errorf("ComputeNoteHash() should detect whitespace changes in body")
	}
}

func TestHashConsistency(t *testing.T) {
	// Ensure hash is always a valid hex string of expected length (SHA256 = 64 hex chars)
	content := `---
created: 2024-01-01
---

# Title

Content`

	hash := ComputeNoteHash(content)

	if len(hash) != 64 {
		t.Errorf("ComputeNoteHash() returned %d chars, want 64", len(hash))
	}

	// Check all characters are valid hex
	for _, ch := range hash {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("ComputeNoteHash() contains non-hex character: %c", ch)
		}
	}
}

func BenchmarkExtractNoteContent(b *testing.B) {
	content := `---
created: 2024-01-01
modified: 2024-01-02
---

# My Title
#tag1 #tag2

Body line 1
Body line 2
Body line 3
Body line 4
Body line 5`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractNoteContent(content)
	}
}

func BenchmarkComputeNoteHash(b *testing.B) {
	content := `---
created: 2024-01-01
modified: 2024-01-02
---

# My Title
#tag1 #tag2

Body line 1
Body line 2
Body line 3`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeNoteHash(content)
	}
}
