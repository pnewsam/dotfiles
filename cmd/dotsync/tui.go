package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type fileAction int

const (
	actionCopy fileAction = iota
	actionSkip
	actionOverwrite
	actionBackup
)

func (a fileAction) label() string {
	switch a {
	case actionCopy:
		return "copy"
	case actionSkip:
		return "skip"
	case actionOverwrite:
		return "overwrite"
	case actionBackup:
		return "backup"
	default:
		return "???"
	}
}

func (a fileAction) tag() string {
	// Fixed-width tag with color codes stripped for alignment.
	// We return raw text and apply color separately in the view.
	switch a {
	case actionCopy:
		return "copy      "
	case actionSkip:
		return "skip      "
	case actionOverwrite:
		return "overwrite "
	case actionBackup:
		return "backup    "
	}
	return "???       "
}

type fileItem struct {
	relPath    string
	srcPath    string
	targetPath string
	srcMode    fs.FileMode
	exists     bool
	same       bool
	action     fileAction
}

func (fi *fileItem) cycleAction() {
	if fi.exists && !fi.same {
		// Conflict: skip → overwrite → backup → skip
		switch fi.action {
		case actionSkip:
			fi.action = actionOverwrite
		case actionOverwrite:
			fi.action = actionBackup
		case actionBackup:
			fi.action = actionSkip
		default:
			fi.action = actionSkip
		}
		return
	}
	if !fi.exists {
		// New file: copy → skip → copy
		if fi.action == actionCopy {
			fi.action = actionSkip
		} else {
			fi.action = actionCopy
		}
	}
}

type tuiModel struct {
	items     []*fileItem
	cursor    int
	quitting  bool
	confirmed bool
	err       error
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}

		case " ", "tab":
			m.items[m.cursor].cycleAction()

		case "enter":
			m.confirmed = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m tuiModel) View() string {
	if m.quitting && !m.confirmed {
		return ""
	}

	if len(m.items) == 0 {
		return c(dim) + "Nothing to sync — all files match." + c(reset) + "\n"
	}

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(c(bold) + "  dotsync" + c(reset))
	if m.confirmed {
		b.WriteString(c(dim) + "  —  applying..." + c(reset))
	}
	b.WriteString("\n\n")

	// Column header
	b.WriteString(c(dim) + "  Action     Path" + c(reset) + "\n")
	b.WriteString(c(dim) + "  ─────────  ────" + c(reset) + "\n")

	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = c(cyan, bold) + "▶ " + c(reset)
		}

		actionStr := item.action.tag()
		pathStr := item.relPath
		note := ""
		if item.exists && !item.same {
			note = c(yellow, dim) + " (differs)" + c(reset)
		} else if !item.exists {
			note = c(dim) + " (new)" + c(reset)
		}

		// Color the action column
		switch item.action {
		case actionCopy:
			actionStr = c(green, bold) + actionStr + c(reset)
		case actionSkip:
			actionStr = c(yellow, bold) + actionStr + c(reset)
		case actionOverwrite:
			actionStr = c(magenta, bold) + actionStr + c(reset)
		case actionBackup:
			actionStr = c(blue, bold) + actionStr + c(reset)
		}

		// Dim entire line for skipped items (when not focused)
		if item.action == actionSkip && i != m.cursor {
			actionStr = c(dim) + item.action.tag() + c(reset)
			pathStr = c(dim) + item.relPath + c(reset)
			note = c(dim) + note + c(reset)
		}

		// Highlight current line's path
		if i == m.cursor && colorEnabled {
			pathStr = c(bold, cyan) + item.relPath + c(reset)
		}

		b.WriteString(fmt.Sprintf("%s%s  %s%s\n", cursor, actionStr, pathStr, note))
	}

	b.WriteString("\n")

	if !m.confirmed {
		b.WriteString(c(dim) + "  ↑↓ navigate   space/tab toggle   enter sync   q quit" + c(reset))
		b.WriteString("\n")
	}

	return b.String()
}

// scanItems walks home/ and builds the file item list for the TUI.
func (s *syncer) scanItems() ([]*fileItem, error) {
	var items []*fileItem

	err := filepath.WalkDir(s.homeSrc, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(s.homeSrc, srcPath)
		if err != nil {
			return fmt.Errorf("relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		if filepath.Base(relPath) == ".DS_Store" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if len(s.filters) > 0 {
			if !matchesFilter(relPath, s.filters) {
				if d.IsDir() && !anyFilterUnder(relPath, s.filters) {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if d.IsDir() {
			return nil
		}

		srcInfo, err := os.Stat(srcPath)
		if err != nil {
			return fmt.Errorf("stat source %s: %w", srcPath, err)
		}

		targetPath := filepath.Join(s.targetRoot, relPath)
		targetInfo, err := os.Stat(targetPath)
		exists := err == nil
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("stat target %s: %w", targetPath, err)
		}

		same := false
		if exists {
			if targetInfo.Size() == srcInfo.Size() {
				same, err = filesEqual(srcPath, targetPath)
				if err != nil {
					return fmt.Errorf("compare %s: %w", relPath, err)
				}
			}
		}

		if exists && same {
			return nil
		}

		item := &fileItem{
			relPath:    relPath,
			srcPath:    srcPath,
			targetPath: targetPath,
			srcMode:    srcInfo.Mode(),
			exists:     exists,
			same:       same,
		}

		if exists && !same {
			item.action = actionSkip
		} else {
			item.action = actionCopy
		}

		items = append(items, item)
		return nil
	})

	return items, err
}