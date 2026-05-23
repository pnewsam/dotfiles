package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	force := flag.Bool("force", false, "Overwrite existing files that differ")
	backup := flag.Bool("backup", false, "Backup existing files before overwriting (renames to .bak)")
	dryRun := flag.Bool("dry-run", false, "Show what would happen without doing it")
	verbose := flag.Bool("verbose", false, "Show all actions, not just warnings")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "dotsync — sync dotfiles from repo to $HOME\n\n")
		fmt.Fprintf(os.Stderr, "Usage: dotsync [flags]\n\n")
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

	s := syncer{
		homeSrc:    homeSrc,
		targetRoot: targetRoot,
		force:      *force,
		backup:     *backup,
		dryRun:     *dryRun,
		verbose:    *verbose,
	}

	if err := s.sync(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// findHomeDir locates the home/ source directory by checking relative to
// the current working directory.
func findHomeDir() string {
	// Check cwd (covers go run and working-dir invocation)
	cwd, err := os.Getwd()
	if err == nil {
		path := filepath.Join(cwd, "home")
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}

	// Check next to the binary (covers installed usage)
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

		targetPath := filepath.Join(s.targetRoot, relPath)

		if d.IsDir() {
			return s.handleDir(targetPath)
		}

		return s.handleFile(srcPath, targetPath, relPath)
	})
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

	// Ensure parent directory exists
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
		// Same size? Do a quick check first; if sizes differ, content differs.
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
		if !s.force && !s.backup {
			fmt.Printf("skip   %s (exists and differs; use --force or --backup)\n", relPath)
			return nil
		}

		if s.backup {
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

		// Reached EOF on both
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

	// Write to a temp file in the same directory, then rename atomically.
	// This avoids leaving a half-written file if something goes wrong.
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".dotsync-tmp-*")
	if err != nil {
		// Fall back to direct write if temp file creation fails
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