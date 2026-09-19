# mcat

High-performance, authentic *The Matrix* digital rain simulator and text viewer for the terminal, written in Go.

[![CI](https://github.com/smford/matrix-cat/actions/workflows/ci.yml/badge.svg)](https://github.com/smford/matrix-cat/actions/workflows/ci.yml)
[![Release](https://github.com/smford/matrix-cat/actions/workflows/release.yml/badge.svg)](https://github.com/smford/matrix-cat/releases)
[![Pages](https://github.com/smford/matrix-cat/actions/workflows/pages.yml/badge.svg)](https://smford.github.io/matrix-cat/)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**Live Demo & Documentation**: [https://smford.github.io/matrix-cat/](https://smford.github.io/matrix-cat/)

---

## Features

- **Authentic Matrix Aesthetics**:
  - Half-width Katakana glyphs (single column width for crisp alignment), numerals, Latin characters, and symbols.
  - Glowing head characters with customizable 24-bit truecolor fading gradients.
  - In-place glyph mutations as characters cascade down the screen.
- **Text File Rain Viewer with Syntax Highlighting**:
  - Pass any text or source code file to have its characters rain down from the top of the screen and settle in place.
  - **Full Syntax Highlighting**: Automatically detects 200+ programming languages (Go, Python, Rust, JavaScript, TypeScript, C/C++, JSON, YAML, Markdown, Bash, etc.) using Chroma.
  - **Vim-Style In-File Search (`/`)**: Search files interactively with `/`, automatically jump to matches, navigate forward with `n`, backward with `p`, and highlight matching occurrences in high-contrast gold and amber.
  - **Live Palette & Syntax Controls**: Toggle between Matrix monochrome and syntax colors with `s`, and cycle syntax themes (Monokai, Dracula, Nord, Solarized Dark, GitHub Dark, Fruity, Native) with `t`.
  - Interactive scrolling (`j`/`k`, arrow keys, PageUp/PageDown, Home/End) for files exceeding terminal dimensions.
  - Fast-forward to instant settled view with `Enter`, or replay rain with `r`.
  - Automatic tab expansion with configurable tab width (`-tabwidth`).
  - Optional screen centering (`-center`) and looping ambient rain (`-loop`).
  - Standard Unix pipeline compatibility: streams raw text when piped to non-TTY outputs.
- **Persistent Configuration (`~/.mcatrc`) & Initialization (`mcat init`)**:
  - Automatically loads personal defaults from `~/.mcatrc` (or custom file via `-config`).
  - Scaffold a complete, well-commented configuration file with `mcat init`.
  - Clean configuration precedence: Built-in Defaults → `~/.mcatrc` → Command-line flags.
- **SRE & Production Engineering Quality**:
  - **Zero Terminal Corruption Guarantee**: Strict cleanup hooks for alternate screen buffer restoration, cursor recovery, and raw mode reset on exit, cancellation, or unexpected panics.
  - **Unbounded Resource Protection**: Guards against memory exhaustion with strict 10MB file limits and streaming safety.
  - **Low Overhead**: Batch rendering with ANSI escape deduplication; minimal CPU and memory allocations even at high framerates.
  - **Dynamic Terminal Resizing**: Real-time terminal geometry adaptation via POSIX `SIGWINCH`.
  - **Non-TTY Protection**: Detects pipes or redirected streams and prevents ANSI escape pollution.
- **Interactive Controls**:
  - Live color palette cycling (Classic Green, Matrix Reloaded Cyan, Amber CRT, Blood Red, Monochrome, Rainbow).
  - Real-time pause, resume, density adjustments, and stream reset.

---

## Installation

### Via Homebrew (Recommended)

Install directly using the [smford/homebrew-tap](https://github.com/smford/homebrew-tap):

```bash
brew tap smford/tap
brew install mcat
```

Or as a single command:

```bash
brew install smford/tap/mcat
```

### Via Go Install

```bash
go install github.com/smford/matrix-cat/cmd/mcat@latest
```

### Pre-built Binaries

Download pre-compiled multi-architecture binaries (macOS Apple Silicon/Intel, Linux ARM64/AMD64, Windows) directly from the [GitHub Releases](https://github.com/smford/matrix-cat/releases) page.

### Build from Source

```bash
git clone https://github.com/smford/matrix-cat.git
cd matrix-cat
make build
```

The executable will be located at `./bin/mcat`.

---

## Usage

### Digital Rain Screensaver

Run with defaults:

```bash
mcat
```

### Displaying Text Files

Pass a text file to have its characters rain down from the top of the terminal in authentic Matrix style:

```bash
mcat myfile.txt
```

You can also pass text via stdin:

```bash
cat myfile.txt | mcat -
```

Or pipe into other tools (`mcat` acts as standard `cat` when stdout is redirected):

```bash
mcat myfile.txt | grep "pattern"
```

### Configuration (`~/.mcatrc`) & `mcat init`

`mcat` supports a persistent configuration file located at `~/.mcatrc`.

#### Generate Default Configuration

To automatically generate a documented `~/.mcatrc` file in your home directory:

```bash
mcat init
```

If `~/.mcatrc` already exists, `mcat init` will protect your existing configuration and notify you. To overwrite it, pass `--force` or `-f`:

```bash
mcat init --force
```

You can also initialize a configuration file at a custom path:

```bash
mcat init -config /path/to/custom.rc
```

#### Example `~/.mcatrc`

```ini
# ~/.mcatrc - Configuration file for mcat
# Generated by 'mcat init'

# Target frame rate per second (10 - 120)
# Default: 30
fps = 30

# Rain drop spawn density percentage (1 - 100)
# Default: 50
density = 50

# Rain falling speed multiplier (0.2 - 3.0)
# Default: 1.0
speed = 1.0

# Rain color scheme palette
# Options: green, cyan, amber, red, white, rainbow
# Default: green
color = green

# Character set
# Options: matrix, ascii, binary, hex
# Default: matrix
charset = matrix

# Highlight leading rain character in bold
# Default: true
bold = true

# Center text horizontally and vertically on screen
# Default: false
center = false

# Keep ambient rain falling continuously around text after settling
# Default: false
loop = false

# Number of spaces for tab expansion (1 - 16)
# Default: 4
tabwidth = 4

# Enable syntax highlighting for code and structured files
# Default: true
syntax = true

# Syntax highlighting theme
# Options: monokai, dracula, nord, solarized-dark, github-dark, fruity, native
# Default: monokai
syntax-theme = monokai
```

#### Configuration Precedence

Settings are resolved in the following order:
1. **Production Defaults** (built into the binary)
2. **Configuration File** (`~/.mcatrc`, or custom `-config <path>`)
3. **CLI Flags** (any flags passed on the command line override file and default settings)

---

### CLI Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-config` | string | `~/.mcatrc` | Path to configuration file |
| `-color` | string | `green` | Palette: `green`, `cyan`, `amber`, `red`, `white`, `rainbow` |
| `-charset` | string | `matrix` | Character set: `matrix`, `ascii`, `binary`, `hex` |
| `-syntax` | bool | `true` | Enable syntax highlighting for code and structured files |
| `-syntax-theme` | string | `monokai` | Theme: `monokai`, `dracula`, `nord`, `solarized-dark`, `github-dark`, `fruity`, `native` |
| `-center` | bool | `false` | Center text horizontally and vertically on screen |
| `-loop` | bool | `false` | Keep ambient rain falling around text after settling |
| `-tabwidth` | int | `4` | Number of spaces for tab expansion (1–16) |
| `-density` | int | `50` | Stream spawn density percentage (1–100) |
| `-fps` | int | `30` | Target frame rate (10–120) |
| `-speed` | float | `1.0` | Speed multiplier (0.2–3.0) |
| `-bold` | bool | `true` | Highlight leading characters in bold |
| `-file` | string | `""` | Path to text file (alternative to positional argument) |
| `-version` | bool | `false` | Display version and build metadata |

#### Examples

```bash
# Rain down source code with Monokai syntax highlighting
mcat main.go

# Rain down code with Dracula syntax theme in cyan rain
mcat -color cyan -syntax-theme dracula app.py

# Centered ASCII banner in CRT amber phosphor
mcat -center -color amber banner.txt

# Disable syntax highlighting for pure monochrome Matrix rain
mcat -syntax=false server.rs

# Read from standard input
cat script.sh | mcat -

# Use a custom configuration file
mcat -config ~/.config/mcat/theme-amber.rc main.go
```

---

## Keyboard Controls

| Key | Action |
|---|---|
| `q`, `Ctrl+C` | Gracefully quit and restore terminal |
| `Esc` | Clear search prompt / highlights, or quit if no active search |
| `/` | Start vim-style in-file search (type pattern + `Enter`) |
| `n` | Jump to next search match |
| `p` / `N` | Jump to previous search match |
| `Space` | Pause / Resume animation |
| `Enter` | Fast-forward rain / settle text immediately |
| `s` | Toggle syntax highlighting on / off |
| `t` | Cycle syntax highlighting themes (Monokai, Dracula, Nord, etc.) |
| `c` | Cycle Matrix rain palettes (Green, Cyan, Amber, Red, White, Rainbow) |
| `+` / `-` | Increase / Decrease speed and density |
| `r` | Reset and replay rain streams |
| `j`, `k`, `↓`, `↑` | Scroll text up / down (when file exceeds screen) |
| `d` / `u`, `PgDn` / `PgUp` | Scroll half page down / up |
| `g` / `G`, `Home` / `End` | Jump to top / bottom of file |

---

## CI/CD, Semantic Versioning & Release Pipeline

1. **Automated Semantic Versioning**:
   - Commits following [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `feat!:`) determine automatic semantic version bumps (`patch`, `minor`, `major`).
   - Manual releases can be triggered via `workflow_dispatch` or git tag push (`git tag vX.Y.Z`).
2. **Multi-Architecture Release Pipeline**:
   - GoReleaser builds and packages `mcat` binaries for Darwin (ARM64 & AMD64), Linux (ARM64 & AMD64), and Windows.
   - Generates SHA256 checksums and automated release notes on GitHub Releases.
3. **Automated Homebrew Tap Publishing**:
   - The GoReleaser pipeline automatically pushes the updated formula and hashes for `mcat` to [smford/homebrew-tap](https://github.com/smford/homebrew-tap).
4. **GitHub Pages Deployment**:
   - Automatically builds and publishes the landing page at `https://smford.github.io/matrix-cat/` from `docs/`.

---

## Testing & Quality Assurance

```bash
# Run unit tests
make test

# Run tests with race detection
make test-race

# Run linter
make lint
```

## Architecture

```
matrix-cat/
├── .github/
│   └── workflows/
│       ├── ci.yml          # Tests, linting, race detection
│       ├── pages.yml       # GitHub Pages automated deployment
│       └── release.yml     # SemVer tagging, GoReleaser, Homebrew publish
├── cmd/
│   └── mcat/
│       ├── main.go         # CLI parsing, ~/.mcatrc loader, 'mcat init', entrypoint
│       └── main_test.go    # CLI flag, pipeline, and init unit tests
├── docs/
│   └── index.html          # Interactive landing page hosted on GitHub Pages
├── Formula/
│   └── mcat.rb             # Reference Homebrew formula for smford/homebrew-tap
├── pkg/
│   └── matrix/
│       ├── chars.go        # Character sets & entropy generation
│       ├── chars_test.go   # Charset unit tests
│       ├── column.go       # Drop state, mutation, and column lifecycle
│       ├── column_test.go  # Column simulation unit tests
│       ├── config.go       # Config struct, ~/.mcatrc parser, generator, validation
│       ├── config_test.go  # Config parser & default generator unit tests
│       ├── matrix.go       # Engine loop, ANSI rendering, vim-search, raw mode
│       ├── matrix_test.go  # Engine & resize unit tests
│       ├── signal_unix.go  # Unix SIGWINCH resize listener
│       ├── signal_windows.go # Windows resize fallback
│       ├── text.go         # TextBuffer, tab expansion, Chroma syntax lexing
│       ├── text_test.go    # TextBuffer, syntax highlighting, and search tests
│       ├── theme.go        # 24-bit Truecolor palettes and gradient calculation
│       └── theme_test.go   # Theme unit tests
├── .goreleaser.yaml        # GoReleaser v2 release and Homebrew formula config
├── Makefile                # Build, test, lint, install
└── README.md
```

## License

MIT