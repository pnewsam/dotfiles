package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	force := flag.Bool("force", false, "Overwrite existing files that differ")
	backup := flag.Bool("backup", false, "Backup existing files before overwriting (renames to .bak)")
	dryRun := flag.Bool("dry-run", false, "Show what would happen without doing it")
	verbose := flag.Bool("verbose", false, "Show all actions, not just warnings")
	interactive := flag.Bool("interactive", false, "Prompt for each conflicting file")
	i := flag.Bool("i", false, "Shorthand for --interactive")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "dotsync — sync dotfiles from repo to $HOME\n\n")
		fmt.Fprintf(os.Stderr, "Usage: dotsync [flags] [path...]\n\n")
		fmt.Fprintf(os.Stderr, "Paths filter to specific files/directories under home/ to sync.\n")
		fmt.Fprintf(os.Stderr, "If no paths are given, everything under home/ is synced.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	homeSrc := findHomeDir()
	if homeSrc == "" {
		fmt.Fprintln(os.Stderr, "error: could not find 'home/' directory")
		os.Exit(1)
	}

	targetRoot := os.Getenv("HOME")
	if targetRoot == "" {
		fmt.Fprintln(os.Stderr, "error: $HOME is not set")
		os.Exit(1)
	}

	filters := normalizeFilters(flag.Args())

	s := syncer{
		homeSrc:     homeSrc,
		targetRoot:  targetRoot,
		force:       *force || *backup,
		backup:      *backup,
		dryRun:      *dryRun,
		verbose:     *verbose,
		interactive: *interactive || *i,
		filters:     filters,
		in:          os.Stdin,
	}

	if err := s.sync(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// normalizeFilters converts CLI args to clean relative paths with no "./" or
// "../" noise, all rooted at the home/ source.
func normalizeFilters(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	filters := make([]string, 0, len(args))
	for _, a := range args {
		a = filepath.Clean(a)
		if a == "." {
			return nil // "sync everything" equivalent
		}
		filters = append(filters, a)
	}
	return filters
}

// matchesFilter returns true if relPath falls under one of the allowed paths.
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

// findHomeDir locates the home/ source directory by checking relative to
// the current working directory.
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

type conflictAction int

const (
	actSkip       conflictAction = iota
	actOverwrite
	actBackup
	actSkipAll
	actOverwriteAll
	actQuit
)

type syncer struct {
	homeSrc     string
	targetRoot  string
	force       bool
	backup      bool
	dryRun      bool
	verbose     bool
	interactive bool
	filters     []string

	// interactive state
	skipAll  bool
	forceAll bool
	quit     bool

	in io.Reader // for prompts (os.Stdin in production)
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

		// Skip the root itself
		if relPath == "." {
			return nil
		}

		// Skip junk files
		base := filepath.Base(relPath)
		if base == ".DS_Store" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// In quit state, bail out of the walk
		if s.quit {
			return filepath.SkipAll
		}

		// If filters are active, skip this entry and possibly prune directories
		if len(s.filters) > 0 {
			if !matchesFilter(relPath, s.filters) {
				if d.IsDir() {
					// Prune sub-tree if no filter targets anything inside it
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

// anyFilterUnder returns true if any filter is rooted under dir.
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
		fmt.Printf("mkdir  %s\n", targetPath)
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
			fmt.Printf("mkdir  %s\n", parent)
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
					fmt.Printf("ok     %s\n", relPath)
				}
				return nil
			}
		}

		// Content differs
		if s.interactive {
			action, err := s.promptConflict(relPath, srcPath, targetPath, srcInfo, targetInfo)
			if err != nil {
				return err
			}
			switch action {
			case actSkip:
				fmt.Printf("skip   %s\n", relPath)
				return nil
			case actOverwrite:
				fmt.Printf("overwrite %s\n", relPath)
			case actBackup:
				backupPath := targetPath + ".bak"
				fmt.Printf("backup %s → %s\n", targetPath, backupPath)
				if !s.dryRun {
					if err := os.Rename(targetPath, backupPath); err != nil {
						return fmt.Errorf("backing up %s: %w", targetPath, err)
					}
				}
			case actSkipAll:
				s.skipAll = true
				fmt.Printf("skip   %s (skipping all remaining)\n", relPath)
				return nil
			case actOverwriteAll:
				s.forceAll = true
				fmt.Printf("overwrite %s (overwriting all remaining)\n", relPath)
			case actQuit:
				s.quit = true
				fmt.Printf("skip   %s (quitting)\n", relPath)
				return nil
			}
		} else if s.skipAll {
			fmt.Printf("skip   %s\n", relPath)
			return nil
		} else if s.forceAll {
			fmt.Printf("overwrite %s\n", relPath)
		} else if !s.force {
			fmt.Printf("skip   %s (exists and differs; use --force, --backup, or -i)\n", relPath)
			return nil
		} else if s.backup {
			backupPath := targetPath + ".bak"
			fmt.Printf("backup %s → %s\n", targetPath, backupPath)
			if !s.dryRun {
				if err := os.Rename(targetPath, backupPath); err != nil {
					return fmt.Errorf("backing up %s: %w", targetPath, err)
				}
			}
		} else {
			fmt.Printf("overwrite %s\n", relPath)
		}
	} else {
		fmt.Printf("copy   %s\n", relPath)
	}

	if !s.dryRun {
		return copyFile(srcPath, targetPath, srcInfo.Mode())
	}

	return nil
}

func (s *syncer) promptConflict(relPath, srcPath, targetPath string, srcInfo, targetInfo fs.FileInfo) (conflictAction, error) {
	scanner := bufio.NewScanner(s.in)

	for {
		fmt.Printf("\n  %s  (exists and differs)\n", relPath)
		fmt.Printf("    repo: %d bytes  |  home: %d bytes\n", srcInfo.Size(), targetInfo.Size())
		fmt.Printf("  [s]kip  [o]verwrite  [b]ackup  [d]iff  [S]kip all  [O]verwrite all  [q]uit\n")
		fmt.Printf("  Choice: ")

		if !scanner.Scan() {
			return actSkip, scanner.Err()
		}

		input := strings.TrimSpace(scanner.Text())

		switch input {
		case "s":
			return actSkip, nil
		case "o":
			return actOverwrite, nil
		case "b":
			return actBackup, nil
		case "d":
			s.showDiff(srcPath, targetPath)
		case "S":
			return actSkipAll, nil
		case "O":
			return actOverwriteAll, nil
		case "q":
			return actQuit, nil
		default:
			fmt.Printf("  Invalid choice: %q\n", input)
		}
	}
}

func (s *syncer) showDiff(srcPath, targetPath string) {
	cmd := exec.Command("diff", "-u", targetPath, srcPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// diff exits 1 when files differ — that's expected, output was shown
			if exitErr.ExitCode() == 1 {
				return
			}
		}
		fmt.Printf("  (diff failed: %v)\n", err)
	}
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