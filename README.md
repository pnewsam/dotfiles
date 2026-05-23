# Dotfiles

My dotfiles, managed with `dotsync`.

## Structure

```
home/           ← mirrors $HOME — this is what gets synced
├── .zshrc
├── .gitignore_global
└── .config/
    └── nvim/
cmd/dotsync/    ← the sync tool
```

## dotsync

Syncs dotfiles from the `home/` directory into `$HOME`.

```bash
# Dry run — see what would happen
go run ./cmd/dotsync --dry-run

# Install dotfiles (skips existing files that differ)
go run ./cmd/dotsync

# Overwrite existing files
go run ./cmd/dotsync --force

# Backup existing files before overwriting
go run ./cmd/dotsync --backup

# Verbose output
go run ./cmd/dotsync --verbose
```

### Conflict behavior

| Situation | Default | `--force` | `--backup` |
|---|---|---|---|
| Target doesn't exist | Copy | Copy | Copy |
| Target exists, identical | Skip | Skip | Skip |
| Target exists, differs | Skip + warn | Overwrite | Rename → `.bak`, then copy |

### Install as a binary

```bash
go install ./cmd/dotsync
```

Then run `dotsync` from any directory inside the repo.

## Notes to Self

Projects to check out:

- [mise](https://mise.jdx.dev) - Version manager
- [jujutsu](https://jj-vcs.github.io/jj/latest/) - Version control
- [just](https://just.systems/) - Command runner
- [nixos](https://nixos.org/)