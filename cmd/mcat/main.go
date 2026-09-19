package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/smford/matrix-cat/pkg/matrix"
	"golang.org/x/term"
)

var (
	version = "1.0.0"
	commit  = "dev"
	date    = "unknown"
)

const maxFileSize = 10 * 1024 * 1024 // 10MB safety limit

func main() {
	// Handle 'mcat init' sub-command
	if len(os.Args) > 1 && os.Args[1] == "init" {
		code := runInit(os.Args[2:], os.Stdout, os.Stderr)
		os.Exit(code)
	}

	fs := flag.NewFlagSet("mcat", flag.ExitOnError)

	fps := fs.Int("fps", 30, "Target frame rate per second (10-120)")
	density := fs.Int("density", 50, "Rain drop spawn density percentage (1-100)")
	speed := fs.Float64("speed", 1.0, "Rain speed multiplier (0.2 - 3.0)")
	themeName := fs.String("color", "green", "Color scheme: green, cyan, amber, red, white, rainbow")
	charSetName := fs.String("charset", "matrix", "Character set: matrix, ascii, binary, hex")
	boldHead := fs.Bool("bold", true, "Highlight leading rain character in bold")
	center := fs.Bool("center", false, "Center text horizontally and vertically on screen")
	loop := fs.Bool("loop", false, "Keep ambient rain falling continuously around text")
	tabWidth := fs.Int("tabwidth", 4, "Number of spaces for tab expansion (1-16)")
	syntax := fs.Bool("syntax", true, "Enable syntax highlighting for code and structured files")
	syntaxTheme := fs.String("syntax-theme", "monokai", "Syntax highlighting theme: monokai, dracula, nord, solarized-dark, github-dark, fruity, native")
	configFile := fs.String("config", "", "Path to configuration file (default: ~/.mcatrc)")
	flagFile := fs.String("file", "", "Path to text file to display (or pass as positional argument)")
	showVersion := fs.Bool("version", false, "Display version and build information")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "mcat - Digital Rain Terminal Simulator & Text Viewer\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  mcat [flags] [file]\n")
		fmt.Fprintf(os.Stderr, "  mcat init [-f|--force] [-config <path>]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  init             Initialize default ~/.mcatrc configuration file\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nControls:\n")
		fmt.Fprintf(os.Stderr, "  q, Ctrl+C        : Quit and restore terminal\n")
		fmt.Fprintf(os.Stderr, "  Esc              : Clear search prompt/highlights, or quit if no active search\n")
		fmt.Fprintf(os.Stderr, "  /                : Search text (type query and press Enter)\n")
		fmt.Fprintf(os.Stderr, "  n / p            : Jump to next / previous search match\n")
		fmt.Fprintf(os.Stderr, "  Space            : Pause / Resume animation\n")
		fmt.Fprintf(os.Stderr, "  c                : Cycle Matrix rain palettes\n")
		fmt.Fprintf(os.Stderr, "  s                : Toggle syntax highlighting on / off\n")
		fmt.Fprintf(os.Stderr, "  t                : Cycle syntax highlighting themes\n")
		fmt.Fprintf(os.Stderr, "  + / -            : Increase / Decrease density & speed\n")
		fmt.Fprintf(os.Stderr, "  r                : Reset rain streams\n")
		fmt.Fprintf(os.Stderr, "  Enter            : Fast-forward rain / Settle text immediately\n")
		fmt.Fprintf(os.Stderr, "  j, k, ↓, ↑       : Scroll text up / down (for long files)\n")
		fmt.Fprintf(os.Stderr, "  d, u, PgDn, PgUp : Scroll half page down / up\n")
		fmt.Fprintf(os.Stderr, "  g, G, Home, End  : Jump to top / bottom of file\n")
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *showVersion {
		fmt.Printf("mcat v%s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Track which flags were explicitly set by the user on the command line
	explicitFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	cfg, err := resolveRuntimeConfig(fs, explicitFlags, *configFile, *fps, *density, *speed, *themeName, *charSetName, *boldHead, *center, *loop, *tabWidth, *syntax, *syntaxTheme, *flagFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Unix pipeline handling: if stdout is not a terminal and a file is specified, behave like cat
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		if cfg.FilePath != "" {
			if err := streamFile(cfg.FilePath, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "Error: mcat must be run inside an interactive terminal.")
		os.Exit(1)
	}

	if cfg.FilePath != "" {
		content, err := loadFile(cfg.FilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		cfg.FileContent = content
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	engine := matrix.NewEngine(cfg, os.Stdout)

	// Ensure cleanup on panic
	defer func() {
		if r := recover(); r != nil {
			engine.Cleanup()
			panic(r)
		}
	}()

	if err := engine.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Engine runtime error: %v\n", err)
		os.Exit(1)
	}
}

// resolveRuntimeConfig builds the runtime Config by layering:
// 1. Production defaults
// 2. ~/.mcatrc (or -config file)
// 3. Command-line flags (explicitly passed)
func resolveRuntimeConfig(
	fs *flag.FlagSet,
	explicitFlags map[string]bool,
	customConfigPath string,
	fps int,
	density int,
	speed float64,
	themeName string,
	charSetName string,
	boldHead bool,
	center bool,
	loop bool,
	tabWidth int,
	syntax bool,
	syntaxTheme string,
	flagFile string,
) (matrix.Config, error) {
	cfg := matrix.DefaultConfig()

	// 1. Determine config file to load
	configToLoad := customConfigPath
	explicitConfigFile := customConfigPath != ""

	if configToLoad == "" {
		defaultPath, err := matrix.DefaultConfigPath()
		if err == nil {
			if _, statErr := os.Stat(defaultPath); statErr == nil {
				configToLoad = defaultPath
			}
		}
	}

	// 2. Load config file if present or explicitly requested
	if configToLoad != "" {
		if err := matrix.LoadAndApplyConfig(&cfg, configToLoad); err != nil {
			if explicitConfigFile {
				return cfg, fmt.Errorf("failed to load specified config '%s': %w", configToLoad, err)
			}
			// When using ~/.mcatrc, report syntax/validation errors clearly
			return cfg, fmt.Errorf("error in ~/.mcatrc (%s): %w", configToLoad, err)
		}
	}

	// 3. Apply CLI flag overrides (highest precedence)
	if explicitFlags["fps"] {
		cfg.FPS = fps
	}
	if explicitFlags["density"] {
		cfg.Density = density
	}
	if explicitFlags["speed"] {
		cfg.SpeedScale = speed
	}
	if explicitFlags["color"] {
		cfg.ThemeName = themeName
	}
	if explicitFlags["charset"] {
		cfg.CharSetName = charSetName
	}
	if explicitFlags["bold"] {
		cfg.BoldHead = boldHead
	}
	if explicitFlags["center"] {
		cfg.Center = center
	}
	if explicitFlags["loop"] {
		cfg.Loop = loop
	}
	if explicitFlags["tabwidth"] {
		cfg.TabWidth = tabWidth
	}
	if explicitFlags["syntax"] {
		cfg.SyntaxHighlight = syntax
	}
	if explicitFlags["syntax-theme"] {
		cfg.SyntaxTheme = syntaxTheme
	}

	// File resolution: positional argument takes precedence over -file flag
	filePath := flagFile
	if fs.NArg() > 0 {
		filePath = fs.Arg(0)
	}
	cfg.FilePath = filePath

	return cfg, nil
}

// runInit implements the 'mcat init' sub-command.
func runInit(args []string, out io.Writer, errOut io.Writer) int {
	initFs := flag.NewFlagSet("mcat init", flag.ContinueOnError)
	initFs.SetOutput(errOut)

	force := initFs.Bool("force", false, "Overwrite existing configuration file")
	forceShort := initFs.Bool("f", false, "Overwrite existing configuration file (shorthand)")
	customPath := initFs.String("config", "", "Custom target path for configuration file")

	if err := initFs.Parse(args); err != nil {
		return 1
	}

	targetPath := *customPath
	if targetPath == "" {
		p, err := matrix.DefaultConfigPath()
		if err != nil {
			fmt.Fprintf(errOut, "mcat init error: %v\n", err)
			return 1
		}
		targetPath = p
	}

	shouldForce := *force || *forceShort
	if !shouldForce {
		if _, err := os.Stat(targetPath); err == nil {
			fmt.Fprintf(out, "mcat: %s already exists. Use 'mcat init --force' (or -f) to overwrite.\n", targetPath)
			return 0
		}
	}

	if err := matrix.WriteDefaultConfigFile(targetPath, shouldForce); err != nil {
		fmt.Fprintf(errOut, "mcat init error: %v\n", err)
		return 1
	}

	// Format display path (show ~ if within user's home)
	displayPath := targetPath
	if home, err := os.UserHomeDir(); err == nil {
		if targetPath == home {
			displayPath = "~"
		} else if strings.HasPrefix(targetPath, home+string(filepath.Separator)) {
			displayPath = "~" + targetPath[len(home):]
		}
	}

	fmt.Fprintf(out, "mcat: initialized configuration file at %s\n", displayPath)
	return 0
}

// loadFile validates and reads a file with strict SRE guardrails against unbounded size.
func loadFile(path string) ([]byte, error) {
	if path == "-" {
		reader := io.LimitReader(os.Stdin, maxFileSize+1)
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		if int64(len(data)) > maxFileSize {
			return nil, fmt.Errorf("input exceeds maximum supported size (10MB)")
		}
		return data, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("'%s' is a directory", path)
	}
	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("file '%s' exceeds maximum supported size (10MB)", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read '%s': %w", path, err)
	}
	return data, nil
}

// streamFile reads and writes a file directly to the given writer for non-interactive pipelines.
func streamFile(path string, out io.Writer) error {
	data, err := loadFile(path)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
