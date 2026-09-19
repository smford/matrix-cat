package matrix

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
)

// Key constants for escape sequences.
const (
	keyUp       = 1001
	keyDown     = 1002
	keyPageUp   = 1003
	keyPageDown = 1004
	keyHome     = 1005
	keyEnd      = 1006
)

// Config encapsulates runtime parameters for the Matrix rain simulation.
type Config struct {
	FPS         int
	Density     int     // 1 - 100
	SpeedScale  float64 // Multiplier for drop speeds (e.g., 0.5x - 3.0x)
	ThemeName   string
	CharSetName string
	BoldHead    bool
	FilePath    string  // Optional path to text file
	FileContent []byte  // Optional raw text content
	Center      bool    // Center text on screen
	Loop        bool    // Keep ambient rain falling after text settles
	TabWidth    int     // Width of tab expansion (default 4)
}

// DefaultConfig returns production-ready default settings.
func DefaultConfig() Config {
	return Config{
		FPS:         30,
		Density:     50,
		SpeedScale:  1.0,
		ThemeName:   "green",
		CharSetName: string(CharSetMatrix),
		BoldHead:    true,
		TabWidth:    4,
	}
}

// Cell represents one terminal character grid cell.
type Cell struct {
	Rune  rune
	Color string
}

// Engine drives the matrix rain simulation and rendering loop.
type Engine struct {
	cfg          Config
	mu           sync.Mutex
	width        int
	height       int
	columns      []*Column
	pool         []rune
	theme        Theme
	themeIndex   int
	paused       bool
	writer       *bufio.Writer
	stdinFd      int
	stdoutFd     int
	termState    *term.State
	ttyFile      *os.File
	textBuf      *TextBuffer
	targetGrid   [][]rune
	textDrops    []*TextDrop
	scrollOffset int
}

// NewEngine constructs a matrix simulation engine.
func NewEngine(cfg Config, out io.Writer) *Engine {
	if cfg.FPS <= 0 {
		cfg.FPS = 30
	}
	if cfg.Density <= 0 || cfg.Density > 100 {
		cfg.Density = 50
	}
	if cfg.SpeedScale <= 0 {
		cfg.SpeedScale = 1.0
	}
	if cfg.TabWidth <= 0 {
		cfg.TabWidth = 4
	}

	theme := GetThemeByName(cfg.ThemeName)
	themeIdx := 0
	for i, t := range AllThemes {
		if t.Name == theme.Name {
			themeIdx = i
			break
		}
	}

	var textBuf *TextBuffer
	if len(cfg.FileContent) > 0 {
		textBuf = NewTextBuffer(cfg.FileContent, cfg.TabWidth)
	}

	return &Engine{
		cfg:        cfg,
		pool:       GetCharPool(CharSet(cfg.CharSetName)),
		theme:      theme,
		themeIndex: themeIdx,
		writer:     bufio.NewWriterSize(out, 64*1024), // 64KB render buffer
		stdinFd:    int(os.Stdin.Fd()),
		stdoutFd:   int(os.Stdout.Fd()),
		textBuf:    textBuf,
	}
}

// isTextMode returns true if engine was configured with text file content.
func (e *Engine) isTextMode() bool {
	return e.textBuf != nil
}

// Run starts the engine in full-screen terminal raw mode, blocking until exit signal or user quit.
func (e *Engine) Run(ctx context.Context) error {
	// Terminal initialization
	stdinFd := e.stdinFd
	if term.IsTerminal(stdinFd) {
		oldState, err := term.MakeRaw(stdinFd)
		if err != nil {
			return fmt.Errorf("failed to enable terminal raw mode: %w", err)
		}
		e.termState = oldState
	} else if tty, err := os.Open("/dev/tty"); err == nil {
		if term.IsTerminal(int(tty.Fd())) {
			oldState, err := term.MakeRaw(int(tty.Fd()))
			if err == nil {
				e.termState = oldState
				e.stdinFd = int(tty.Fd())
				e.ttyFile = tty
			}
		}
	}

	// Defensive cleanup guarantee: restore terminal state on return or panic
	defer e.Cleanup()

	// Switch to alternate screen buffer, hide cursor, clear screen
	_, _ = fmt.Fprint(e.writer, "\x1b[?1049h\x1b[?25l\x1b[2J")
	_ = e.writer.Flush()

	// Initial dimensions
	w, h, err := term.GetSize(e.stdoutFd)
	if err != nil || w <= 0 || h <= 0 {
		w, h = 80, 24
	}
	e.resize(w, h)

	// OS signal handling
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Window resize signal (SIGWINCH)
	winchChan := make(chan os.Signal, 2)
	notifyResize(winchChan)

	// Non-blocking keyboard input stream
	keyChan := make(chan int, 16)
	keyReader := io.Reader(os.Stdin)
	if e.ttyFile != nil {
		keyReader = e.ttyFile
	}
	go e.listenKeys(keyReader, keyChan)

	// Simulation ticker
	frameDuration := time.Second / time.Duration(e.cfg.FPS)
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-sigChan:
			return nil

		case <-winchChan:
			nw, nh, err := term.GetSize(e.stdoutFd)
			if err == nil && nw > 0 && nh > 0 {
				e.mu.Lock()
				e.resize(nw, nh)
				e.mu.Unlock()
			}

		case key := <-keyChan:
			if e.handleInput(key) {
				return nil // Exit requested
			}

		case <-ticker.C:
			e.mu.Lock()
			if !e.paused {
				e.step()
			}
			e.render()
			e.mu.Unlock()
		}
	}
}

// resize updates internal dimensions and adapts column slices or text grids.
func (e *Engine) resize(newW, newH int) {
	e.width = newW
	e.height = newH

	if e.isTextMode() {
		e.rebuildTargetGrid()
		e.rebuildTextDrops()
	} else {
		currentCols := len(e.columns)
		if newW > currentCols {
			for i := currentCols; i < newW; i++ {
				e.columns = append(e.columns, NewColumn())
			}
		} else if newW < currentCols {
			e.columns = e.columns[:newW]
		}
	}
}

// maxScrollOffset returns the maximum allowed scroll offset given current height.
func (e *Engine) maxScrollOffset() int {
	if e.textBuf == nil {
		return 0
	}
	total := e.textBuf.TotalLines(e.width)
	if total <= e.height {
		return 0
	}
	return total - e.height
}

// rebuildTargetGrid computes target rune positions for the current visible window.
func (e *Engine) rebuildTargetGrid() {
	if e.textBuf == nil || e.width <= 0 || e.height <= 0 {
		return
	}

	wrapped := e.textBuf.Wrap(e.width)
	numLines := len(wrapped)

	maxOffset := e.maxScrollOffset()
	if e.scrollOffset > maxOffset {
		e.scrollOffset = maxOffset
	}
	if e.scrollOffset < 0 {
		e.scrollOffset = 0
	}

	end := e.scrollOffset + e.height
	if end > numLines {
		end = numLines
	}

	var visible [][]rune
	if e.scrollOffset < numLines {
		visible = wrapped[e.scrollOffset:end]
	}

	startRow := 0
	startCol := 0

	if e.cfg.Center {
		maxLen := 0
		for _, line := range visible {
			if len(line) > maxLen {
				maxLen = len(line)
			}
		}
		startRow = (e.height - len(visible)) / 2
		if startRow < 0 {
			startRow = 0
		}
		startCol = (e.width - maxLen) / 2
		if startCol < 0 {
			startCol = 0
		}
	}

	target := make([][]rune, e.height)
	for r := range target {
		target[r] = make([]rune, e.width)
	}

	for i, line := range visible {
		r := startRow + i
		if r >= e.height {
			break
		}
		for j, ch := range line {
			c := startCol + j
			if c >= e.width {
				break
			}
			if ch != ' ' {
				target[r][c] = ch
			}
		}
	}

	e.targetGrid = target
}

// rebuildTextDrops adjusts drop array to current terminal width.
func (e *Engine) rebuildTextDrops() {
	if e.width <= 0 || e.height <= 0 {
		return
	}
	currentDrops := len(e.textDrops)
	if currentDrops == 0 {
		e.resetTextRain()
		return
	}

	if e.width > currentDrops {
		for i := currentDrops; i < e.width; i++ {
			e.textDrops = append(e.textDrops, NewTextDrop(e.height, e.pool, 15))
		}
	} else if e.width < currentDrops {
		e.textDrops = e.textDrops[:e.width]
	}
}

// resetTextRain resets drops to start raining text from the top.
func (e *Engine) resetTextRain() {
	if e.width <= 0 || e.height <= 0 {
		return
	}
	e.rebuildTargetGrid()
	e.textDrops = make([]*TextDrop, e.width)
	for x := 0; x < e.width; x++ {
		e.textDrops[x] = NewTextDrop(e.height, e.pool, 25)
	}
}

// fastForwardTextRain completes all active drops instantly, settling all characters.
func (e *Engine) fastForwardTextRain() {
	for _, d := range e.textDrops {
		if d != nil {
			d.Y = float64(e.height + d.Length + 10)
			d.Done = true
		}
	}
}

// allTextDropsDone reports whether all columns have finished raining down.
func (e *Engine) allTextDropsDone() bool {
	if len(e.textDrops) == 0 {
		return true
	}
	for _, d := range e.textDrops {
		if d != nil && !d.Done {
			return false
		}
	}
	return true
}

// scrollDown scrolls the visible text downward.
func (e *Engine) scrollDown(lines int) {
	if lines <= 0 {
		lines = 1
	}
	maxOffset := e.maxScrollOffset()
	if e.scrollOffset < maxOffset {
		e.scrollOffset += lines
		if e.scrollOffset > maxOffset {
			e.scrollOffset = maxOffset
		}
		e.rebuildTargetGrid()
	}
}

// scrollUp scrolls the visible text upward.
func (e *Engine) scrollUp(lines int) {
	if lines <= 0 {
		lines = 1
	}
	if e.scrollOffset > 0 {
		e.scrollOffset -= lines
		if e.scrollOffset < 0 {
			e.scrollOffset = 0
		}
		e.rebuildTargetGrid()
	}
}

// scrollTo scrolls to a specific line offset.
func (e *Engine) scrollTo(offset int) {
	maxOffset := e.maxScrollOffset()
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if e.scrollOffset != offset {
		e.scrollOffset = offset
		e.rebuildTargetGrid()
	}
}

// step advances all simulation streams by one frame.
func (e *Engine) step() {
	if e.isTextMode() {
		e.stepTextMode()
	} else {
		for _, col := range e.columns {
			col.Update(e.height, e.pool, e.cfg.Density, e.cfg.SpeedScale)
		}
	}
}

// stepTextMode advances text drops and handles looping ambient rain if configured.
func (e *Engine) stepTextMode() {
	allDone := true
	for _, d := range e.textDrops {
		if d != nil && !d.Done {
			d.Update(e.pool, e.cfg.SpeedScale, e.height)
			if !d.Done {
				allDone = false
			}
		}
	}

	if e.cfg.Loop && allDone {
		spawnChance := e.cfg.Density / 4
		if spawnChance < 1 {
			spawnChance = 1
		}
		for x := 0; x < e.width; x++ {
			if e.textDrops[x] == nil || e.textDrops[x].Done {
				if randomInt(1, 100) <= spawnChance {
					e.textDrops[x] = NewTextDrop(e.height, e.pool, 5)
				}
			}
		}
	}
}

// render computes the frame buffer and flushes optimized ANSI output.
func (e *Engine) render() {
	if e.width <= 0 || e.height <= 0 {
		return
	}

	if e.isTextMode() {
		e.renderTextMode()
	} else {
		e.renderScreensaver()
	}
}

// renderScreensaver renders continuous screensaver rain.
func (e *Engine) renderScreensaver() {
	grid := make([][]Cell, e.height)
	for r := range grid {
		grid[r] = make([]Cell, e.width)
	}

	for x, col := range e.columns {
		if x >= e.width {
			continue
		}
		for _, drop := range col.Drops {
			headRow := int(drop.Y)
			for dist := 0; dist < drop.Length; dist++ {
				row := headRow - dist
				if row >= 0 && row < e.height {
					if dist < len(drop.Glyphs) {
						color := e.theme.ColorAtPosition(dist, drop.Length, e.cfg.BoldHead)
						grid[row][x] = Cell{
							Rune:  drop.Glyphs[dist],
							Color: color,
						}
					}
				}
			}
		}
	}

	e.flushGrid(grid)
}

// renderTextMode renders the text file being revealed through falling characters.
func (e *Engine) renderTextMode() {
	grid := make([][]Cell, e.height)
	for r := range grid {
		grid[r] = make([]Cell, e.width)
	}

	for x := 0; x < e.width; x++ {
		var drop *TextDrop
		if x < len(e.textDrops) {
			drop = e.textDrops[x]
		}

		for y := 0; y < e.height; y++ {
			var targetRune rune
			if y < len(e.targetGrid) && x < len(e.targetGrid[y]) {
				targetRune = e.targetGrid[y][x]
			}

			if drop == nil || drop.Done {
				// Settled state for this cell
				if targetRune != 0 {
					grid[y][x] = Cell{
						Rune:  targetRune,
						Color: e.theme.SettledColor(x),
					}
				}
				continue
			}

			headRow := int(drop.Y)
			tailRow := headRow - drop.Length

			switch {
			case headRow < y:
				// Not yet reached: empty cell

			case headRow == y:
				// Drop head is at this cell
				r := targetRune
				if r == 0 {
					r = drop.Glyphs[0]
				}
				grid[y][x] = Cell{
					Rune:  r,
					Color: e.theme.HeadColor.ANSI(e.cfg.BoldHead),
				}

			case tailRow <= y && y < headRow:
				// Inside trail
				dist := headRow - y
				color := e.theme.ColorAtPosition(dist, drop.Length, false)
				r := targetRune
				if r != 0 {
					// 5% chance to shimmer with matrix rune
					if randomInt(1, 100) <= 5 {
						r = RandomRune(e.pool)
					}
				} else {
					if dist < len(drop.Glyphs) {
						r = drop.Glyphs[dist]
					} else {
						r = RandomRune(e.pool)
					}
				}
				grid[y][x] = Cell{
					Rune:  r,
					Color: color,
				}

			case tailRow > y:
				// Trail has passed: settled
				if targetRune != 0 {
					grid[y][x] = Cell{
						Rune:  targetRune,
						Color: e.theme.SettledColor(x),
					}
				}
			}
		}
	}

	e.flushGrid(grid)
}

// flushGrid outputs the grid to terminal using ANSI color deduplication.
func (e *Engine) flushGrid(grid [][]Cell) {
	_, _ = e.writer.WriteString("\x1b[H")

	currentColor := ""
	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			cell := grid[y][x]
			if cell.Rune == 0 || cell.Rune == ' ' {
				if currentColor != "" {
					_, _ = e.writer.WriteString("\x1b[0m")
					currentColor = ""
				}
				_ = e.writer.WriteByte(' ')
			} else {
				if cell.Color != currentColor {
					_, _ = e.writer.WriteString(cell.Color)
					currentColor = cell.Color
				}
				_, _ = e.writer.WriteString(string(cell.Rune))
			}
		}
		if currentColor != "" && y < e.height-1 {
			_, _ = e.writer.WriteString("\x1b[0m")
			currentColor = ""
		}
		if y < e.height-1 {
			_, _ = e.writer.WriteString("\r\n")
		}
	}

	if currentColor != "" {
		_, _ = e.writer.WriteString("\x1b[0m")
	}

	_ = e.writer.Flush()
}

// handleInput processes interactive single-key commands. Returns true to signal exit.
func (e *Engine) handleInput(key int) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch key {
	case 'q', 'Q', 3, 27: // 'q', 'Q', Ctrl+C (0x03), ESC (0x1b)
		return true

	case '\r', '\n': // Enter key
		if e.isTextMode() {
			if !e.allTextDropsDone() {
				e.fastForwardTextRain()
			}
		}

	case ' ': // Pause / resume
		e.paused = !e.paused

	case 'c', 'C': // Cycle theme
		e.themeIndex = (e.themeIndex + 1) % len(AllThemes)
		e.theme = AllThemes[e.themeIndex]

	case '+', '=': // Increase speed & density
		if e.cfg.SpeedScale < 3.0 {
			e.cfg.SpeedScale += 0.2
		}
		if e.cfg.Density <= 90 {
			e.cfg.Density += 10
		}

	case '-', '_': // Decrease speed & density
		if e.cfg.SpeedScale > 0.3 {
			e.cfg.SpeedScale -= 0.2
		}
		if e.cfg.Density >= 20 {
			e.cfg.Density -= 10
		}

	case 'r', 'R': // Reset rain
		if e.isTextMode() {
			e.resetTextRain()
		} else {
			for i := range e.columns {
				e.columns[i] = NewColumn()
			}
		}

	case 'j', 'J', keyDown:
		if e.isTextMode() {
			e.scrollDown(1)
		}

	case 'k', 'K', keyUp:
		if e.isTextMode() {
			e.scrollUp(1)
		}

	case 'd', 'D', keyPageDown:
		if e.isTextMode() {
			e.scrollDown(e.height / 2)
		}

	case 'u', 'U', keyPageUp:
		if e.isTextMode() {
			e.scrollUp(e.height / 2)
		}

	case 'g', keyHome:
		if e.isTextMode() {
			e.scrollTo(0)
		}

	case 'G', keyEnd:
		if e.isTextMode() {
			e.scrollTo(e.maxScrollOffset())
		}
	}

	return false
}

// listenKeys streams keystrokes and decodes multi-byte escape sequences.
func (e *Engine) listenKeys(reader io.Reader, ch chan<- int) {
	buf := make([]byte, 32)
	for {
		n, err := reader.Read(buf)
		if err != nil || n == 0 {
			return
		}
		if n == 1 {
			ch <- int(buf[0])
			continue
		}
		if buf[0] == 0x1b && n >= 3 {
			if buf[1] == '[' || buf[1] == 'O' {
				switch buf[2] {
				case 'A':
					ch <- keyUp
					continue
				case 'B':
					ch <- keyDown
					continue
				case 'H':
					ch <- keyHome
					continue
				case 'F':
					ch <- keyEnd
					continue
				case '5':
					if n >= 4 && buf[3] == '~' {
						ch <- keyPageUp
						continue
					}
				case '6':
					if n >= 4 && buf[3] == '~' {
						ch <- keyPageDown
						continue
					}
				case '1':
					if n >= 4 && buf[3] == '~' {
						ch <- keyHome
						continue
					}
				case '4':
					if n >= 4 && buf[3] == '~' {
						ch <- keyEnd
						continue
					}
				}
			}
		}
		ch <- int(buf[0])
	}
}

// Cleanup restores the terminal to standard mode, leaves alternate buffer, and unhides cursor.
func (e *Engine) Cleanup() {
	e.mu.Lock()
	defer e.mu.Unlock()

	_, _ = fmt.Fprint(e.writer, "\x1b[0m\x1b[?25h\x1b[?1049l")
	_ = e.writer.Flush()

	if e.termState != nil {
		_ = term.Restore(e.stdinFd, e.termState)
		e.termState = nil
	}

	if e.ttyFile != nil {
		_ = e.ttyFile.Close()
		e.ttyFile = nil
	}
}
