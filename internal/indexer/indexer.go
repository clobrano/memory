package indexer

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/clobrano/memory/internal/db"
)

type Note struct {
	Path    string
	Title   string
	Tag     string
	Content string
}

func Scan(dirs []string, tags []string) ([]Note, error) {
	var (
		mu    sync.Mutex
		notes []Note
	)

	paths, err := collectMarkdownPaths(dirs)
	if err != nil {
		return nil, err
	}

	sem := make(chan struct{}, runtime.NumCPU())
	g, _ := errgroup.WithContext(context.Background())

	for _, p := range paths {
		p := p
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			note, ok, err := processFile(p, tags)
			if err != nil || !ok {
				return err
			}
			mu.Lock()
			notes = append(notes, note)
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return notes, nil
}

func collectMarkdownPaths(dirs []string) ([]string, error) {
	var paths []string
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable dirs
			}
			if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func processFile(path string, tags []string) (Note, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Note{}, false, err
	}
	// binary heuristic: presence of null byte
	if bytes.IndexByte(data, 0) >= 0 {
		return Note{}, false, nil
	}

	content := string(data)
	tag := findTag(content, tags)
	if tag == "" {
		return Note{}, false, nil
	}

	title := extractTitle(content, path)
	return Note{Path: path, Title: title, Tag: tag, Content: content}, true, nil
}

func findTag(content string, tags []string) string {
	for _, tag := range tags {
		if strings.Contains(content, tag) {
			return tag
		}
	}
	return ""
}

func extractTitle(content, path string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func Diff(scanned []Note, stored []db.Card) (newNotes []Note, missing []db.Card) {
	storedByPath := make(map[string]bool, len(stored))
	for _, c := range stored {
		storedByPath[c.Path] = true
	}

	scannedPaths := make(map[string]bool, len(scanned))
	for _, n := range scanned {
		scannedPaths[n.Path] = true
		if !storedByPath[n.Path] {
			newNotes = append(newNotes, n)
		}
	}

	for _, c := range stored {
		if !scannedPaths[c.Path] {
			missing = append(missing, c)
		}
	}
	return
}
