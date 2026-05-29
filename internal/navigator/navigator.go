package navigator

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultPageSize = 12

type DirEntry struct {
	Name  string
	IsDir bool
}

type Page struct {
	Entries    []DirEntry
	Path       string
	PageNum    int
	TotalPages int
	PageSize   int
}

func ListDir(path string, page, pageSize int) (*Page, error) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", absPath)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	items := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		items = append(items, DirEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	totalPages := int(math.Ceil(float64(len(items)) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * pageSize
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}

	return &Page{
		Entries:    items[start:end],
		Path:       absPath,
		PageNum:    page,
		TotalPages: totalPages,
		PageSize:   pageSize,
	}, nil
}
