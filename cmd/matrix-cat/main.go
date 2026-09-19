package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
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
	fps := flag.Int("fps", 30, "Target frame rate per second (10-120)")
	density := flag.Int("density", 50, "Rain drop spawn density percentage (1-100)")
	speed := flag.Float64("speed", 1.0, "Rain speed multiplier (0.2 - 3.0)")
	themeName := flag.String("color", "green", "Color scheme: green, cyan, amber, red, white, rainbow")
	charSetName := flag.String("charset", "matrix", "Character set: matrix, ascii, binary, hex")
	boldHead := flag.Bool("bold", true, "Highlight leading rain character in bold")
	center := flag.Bool("center", false, "Center text horizontally and vertically on screen")
	loop := flag.Bool("loop", false, "Keep ambient rain falling continuously around text")
	tabWidth := flag.Int("tabwidth", 4, "Number of spaces for tab expansion (1-16)")
	syntax := flag.Bool("syntax", true, "Enable syntax highlighting for code and structured files")
	syntaxTheme := flag.String("syntax-theme", "monokai", "Syntax highlighting theme: monokai, dracula, nord, solarized-dark, github-dark, fruity, native")
	flagFile := flag.String("file", "", "Path to text file to display (or pass as positional argument)")
	showVersion := flag.Bool("version", false, "Display version and build information")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Matrix Cat - Digital Rain Terminal Simulator & Text Viewer\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [file]\n\nFlags:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nControls:\n")
		fmt.Fprintf(os.Stderr, "  q, Esc, Ctrl+C : Quit\n")
		fmt.Fprintf(os.Stderr, "  Space          : Pause / Resume animation\n")
		fmt.Fprintf(os.Stderr, "  c              : Cycle Matrix rain palettes\n")
		fmt.Fprintf(os.Stderr, "  s              : Toggle syntax highlighting on / off\n")
		fmt.Fprintf(os.Stderr, "  t              : Cycle syntax highlighting themes\n")
		fmt.Fprintf(os.Stderr, "  + / -          : Increase / Decrease density & speed\n")
		fmt.Fprintf(os.Stderr, "  r              : Reset rain streams\n")
		fmt.Fprintf(os.Stderr, "  Enter          : Fast-forward rain / Settle text immediately\n")
		fmt.Fprintf(os.Stderr, "  j, k, ↓, ↑     : Scroll text up / down (for long files)\n")
		fmt.Fprintf(os.Stderr, "  d, u, PgDn, PgUp : Scroll half page down / up\n")
		fmt.Fprintf(os.Stderr, "  g, G, Home, End  : Jump to top / bottom of file\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("matrix-cat v%s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Resolve file path from positional argument or -file flag
	filePath := *flagFile
	if flag.NArg() > 0 {
		filePath = flag.Arg(0)
	}

	// Unix pipeline handling: if stdout is not a terminal and a file is specified, behave like cat
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		if filePath != "" {
			if err := streamFile(filePath, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "Error: matrix-cat must be run inside an interactive terminal.")
		os.Exit(1)
	}

	var fileContent []byte
	if filePath != "" {
		content, err := loadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fileContent = content
	}

	cfg := matrix.Config{
		FPS:         *fps,
		Density:     *density,
		SpeedScale:  *speed,
		ThemeName:   *themeName,
		CharSetName: *charSetName,
		BoldHead:    *boldHead,
		FilePath:    filePath,
		FileContent: fileContent,
		Center:          *center,
		Loop:            *loop,
		TabWidth:        *tabWidth,
		SyntaxHighlight: *syntax,
		SyntaxTheme:     *syntaxTheme,
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
