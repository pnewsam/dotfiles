package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ANSI escapes.
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
)

var colorEnabled = true

func c(codes ...string) string {
	if !colorEnabled {
		return ""
	}
	return strings.Join(codes, "")
}

func main() {
	force := flag.Bool("force", false, "Overwrite existing files that differ")
	backup := flag.Bool("backup", false, "Backup existing files before overwriting (renames to .bak)")
	dryRun := flag.Bool("dry-run", false, "Show what would happen without doing it")
	verbose := flag.Bool("verbose", false, "Show all actions, not just warnings")
	interactive := flag.Bool("interactive", false, "Interactive TUI to select files and actions")
	iFlag := flag.Bool("i", false, "Shorthand for --interactive")
	noColor := flag.Bool("no-color", false, "Disable colored output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "dotsync — sync dotfiles from repo to $HOME\n\n")
		fmt.Fprintf(os.Stderr, "Usage: dotsync [flags] [path...]\n\n")
		fmt.Fprintf(os.Stderr, "Paths filter to specific files/directories under home/ to sync.\n")
		fmt.Fprintf(os.Stderr, "If no paths are given, everything under home/ is synced.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *noColor {
		colorEnabled = false
	}

	homeSrc := findHomeDir()
	if homeSrc == "" {
		fmt.Fprintln(os.Stderr, c(red, bold)+"error:"+c(reset)+" could not find 'home/' directory")
		os.Exit(1)
	}

	targetRoot := os.Getenv("HOME")
	if targetRoot == "" {
		fmt.Fprintln(os.Stderr, c(red, bold)+"error:"+c(reset)+" $HOME is not set")
		os.Exit(1)
	}

	filters := normalizeFilters(flag.Args())

	s := syncer{
		homeSrc:    homeSrc,
		targetRoot: targetRoot,
		force:      *force || *backup,
		backup:     *backup,
		dryRun:     *dryRun,
		verbose:    *verbose,
		filters:    filters,
	}

	if *interactive || *iFlag {
		if err := s.runTUI(); err != nil {
			fmt.Fprintf(os.Stderr, c(red, bold)+"error:"+c(reset)+" %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := s.sync(); err != nil {
		fmt.Fprintf(os.Stderr, c(red, bold)+"error:"+c(reset)+" %v\n", err)
		os.Exit(1)
	}
}

func (s *syncer) runTUI() error {
	items, err := s.scanItems()
	if err != nil {
		return err
	}

	if len(items) == 0 {
		fmt.Println("Nothing to sync — all files match.")
		return nil
	}

	model := tuiModel{items: items}
	p := tea.NewProgram(model, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}

	m := final.(tuiModel)
	if !m.confirmed {
		fmt.Println("Cancelled.")
		return nil
	}

	return s.applyItems(m.items)
}

func (s *syncer) applyItems(items []*fileItem) error {
	for _, item := range items {
		switch item.action {
		case actionSkip:
			s.printSkip(item.relPath, "")
		case actionCopy:
			fmt.Printf("%s %s\n",
				c(green, bold)+"copy"+c(reset)+"   ",
				c(cyan)+item.relPath+c(reset))
			if !s.dryRun {
				// Ensure parent dir exists
				parent := filepath.Dir(item.targetPath)
				if _, err := os.Stat(parent); os.IsNotExist(err) {
					if err := os.MkdirAll(parent, 0755); err != nil {
						return fmt.Errorf("mkdir %s: %w", parent, err)
					}
				}
				if err := copyFile(item.srcPath, item.targetPath, item.srcMode); err != nil {
					return fmt.Errorf("copy %s: %w", item.relPath, err)
				}
			}
		case actionOverwrite:
			s.printOverwrite(item.relPath, "")
			if !s.dryRun {
				if err := copyFile(item.srcPath, item.targetPath, item.srcMode); err != nil {
					return fmt.Errorf("overwrite %s: %w", item.relPath, err)
				}
			}
		case actionBackup:
			backupPath := item.targetPath + ".bak"
			s.printBackup(item.targetPath, backupPath)
			if !s.dryRun {
				if err := os.Rename(item.targetPath, backupPath); err != nil {
					return fmt.Errorf("backup %s: %w", item.targetPath, err)
				}
				if err := copyFile(item.srcPath, item.targetPath, item.srcMode); err != nil {
					return fmt.Errorf("copy after backup %s: %w", item.relPath, err)
				}
			}
		}
	}
	return nil
}

// normalizeFilters converts CLI args to clean relative paths.
func normalizeFilters(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	filters := make([]string, 0, len(args))
	for _, a := range args {
		a = filepath.Clean(a)
		if a == "." {
			return nil
		}
		filters = append(filters, a)
	}
	return filters
}

func matchesFilter(relPath string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if f == relPath {
			return true
		}
		if strings.HasPrefix(relPath, f+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func findHomeDir() string {
	cwd, err := os.Getwd()
	if err == nil {
		path := filepath.Join(cwd, "home")
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	if exe, err := os.Executable(); err == nil {
		path := filepath.Join(filepath.Dir(exe), "home")
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}

type syncer struct {
	homeSrc    string
	targetRoot string
	force      bool
	backup     bool
	dryRun     bool
	verbose    bool
	filters    []string
}

func (s *syncer) sync() error {
	return filepath.WalkDir(s.homeSrc, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(s.homeSrc, srcPath)
		if err != nil {
			return fmt.Errorf("computing relative path for %s: %w", srcPath, err)
		}

		if relPath == "." {
			return nil
		}

		base := filepath.Base(relPath)
		if base == ".DS_Store" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if len(s.filters) > 0 {
			if !matchesFilter(relPath, s.filters) {
				if d.IsDir() {
					if !anyFilterUnder(relPath, s.filters) {
						return filepath.SkipDir
					}
				}
				return nil
			}
		}

		targetPath := filepath.Join(s.targetRoot, relPath)

		if d.IsDir() {
			return s.handleDir(targetPath)
		}

		return s.handleFile(srcPath, targetPath, relPath)
	})
}

func anyFilterUnder(dir string, filters []string) bool {
	prefix := dir + string(filepath.Separator)
	for _, f := range filters {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

func (s *syncer) handleDir(targetPath string) error {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		fmt.Printf("%s %s\n",
			c(blue, bold)+"mkdir"+c(reset)+"  ",
			c(cyan)+targetPath+c(reset))
		if !s.dryRun {
			return os.MkdirAll(targetPath, 0755)
		}
	}
	return nil
}

func (s *syncer) handleFile(srcPath, targetPath, relPath string) error {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("stat source %s: %w", srcPath, err)
	}

	targetInfo, err := os.Stat(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat target %s: %w", targetPath, err)
	}

	targetExists := err == nil

	if !targetExists {
		parent := filepath.Dir(targetPath)
		if _, err := os.Stat(parent); os.IsNotExist(err) {
			fmt.Printf("%s %s\n",
				c(blue, bold)+"mkdir"+c(reset)+"  ",
				c(cyan)+parent+c(reset))
			if !s.dryRun {
				if err := os.MkdirAll(parent, 0755); err != nil {
					return fmt.Errorf("create parent dir %s: %w", parent, err)
				}
			}
		}
	}

	if targetExists {
		if targetInfo.Size() == srcInfo.Size() {
			same, err := filesEqual(srcPath, targetPath)
			if err != nil {
				return fmt.Errorf("comparing %s: %w", relPath, err)
			}
			if same {
				if s.verbose {
					fmt.Printf("%s %s\n",
						c(dim, green)+"ok"+c(reset)+"      ",
						c(dim)+relPath+c(reset))
				}
				return nil
			}
		}

		if s.backup {
			backupPath := targetPath + ".bak"
			s.printBackup(targetPath, backupPath)
			if !s.dryRun {
				if err := os.Rename(targetPath, backupPath); err != nil {
					return fmt.Errorf("backing up %s: %w", targetPath, err)
				}
			}
		} else if s.force {
			s.printOverwrite(relPath, "")
		} else {
			s.printSkip(relPath, "exists and differs; use --force, --backup, or -i")
			return nil
		}
	} else {
		fmt.Printf("%s %s\n",
			c(green, bold)+"copy"+c(reset)+"   ",
			c(cyan)+relPath+c(reset))
	}

	if !s.dryRun {
		return copyFile(srcPath, targetPath, srcInfo.Mode())
	}

	return nil
}

func (s *syncer) printSkip(path, note string) {
	if note != "" {
		fmt.Printf("%s %s %s\n",
			c(yellow, bold)+"skip"+c(reset)+"   ",
			c(yellow)+path+c(reset),
			c(dim)+"("+note+")"+c(reset))
	} else {
		fmt.Printf("%s %s\n",
			c(yellow, bold)+"skip"+c(reset)+"   ",
			c(yellow)+path+c(reset))
	}
}

func (s *syncer) printOverwrite(path, note string) {
	if note != "" {
		fmt.Printf("%s %s %s\n",
			c(magenta, bold)+"overwrite"+c(reset)+" ",
			c(cyan)+path+c(reset),
			c(dim)+"("+note+")"+c(reset))
	} else {
		fmt.Printf("%s %s\n",
			c(magenta, bold)+"overwrite"+c(reset)+" ",
			c(cyan)+path+c(reset))
	}
}

func (s *syncer) printBackup(target, backup string) {
	fmt.Printf("%s %s %s %s\n",
		c(magenta, bold)+"backup"+c(reset)+" ",
		c(cyan)+target+c(reset),
		c(dim)+"→"+c(reset),
		c(cyan)+backup+c(reset))
}

func filesEqual(a, b string) (bool, error) {
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()

	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()

	const bufSize = 64 * 1024
	bufA := make([]byte, bufSize)
	bufB := make([]byte, bufSize)

	for {
		na, errA := io.ReadFull(fa, bufA)
		nb, errB := io.ReadFull(fb, bufB)

		if na != nb {
			return false, nil
		}

		if !bytes.Equal(bufA[:na], bufB[:nb]) {
			return false, nil
		}

		if errA == io.EOF || errA == io.ErrUnexpectedEOF {
			return true, nil
		}
		if errB == io.EOF || errB == io.ErrUnexpectedEOF {
			return true, nil
		}

		if errA != nil {
			return false, errA
		}
		if errB != nil {
			return false, errB
		}
	}
}

func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".dotsync-tmp-*")
	if err != nil {
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, in); err != nil {
			return err
		}
		return os.Chmod(dst, mode)
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), dst)
}