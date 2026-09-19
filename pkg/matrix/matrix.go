package matrix

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
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

// SearchMatch represents an occurrence of a search query in wrapped lines.
type SearchMatch struct {
	Index int // 0-based match index
	Line  int // Line index in wrapped lines
	Col   int // Column offset in wrapped line
	Len   int // Match length in runes
}

// TargetCell stores character, syntax color, and document position for a grid cell.
type TargetCell struct {
	Rune    rune
	Color   string
	DocLine int
	Col     int
}

// Config encapsulates runtime parameters for the Matrix rain simulation.
type Config struct {
	FPS             int
	Density         int     // 1 - 100
	SpeedScale      float64 // Multiplier for drop speeds (e.g., 0.5x - 3.0x)
	ThemeName       string
	CharSetName     string
	BoldHead        bool
	FilePath        string  // Optional path to text file
	FileContent     []byte  // Optional raw text content
	Center          bool    // Center text on screen
	Loop            bool    // Keep ambient rain falling after text settles
	TabWidth        int     // Width of tab expansion (default 4)
	SyntaxHighlight bool    // Enable syntax highlighting (default true)
	SyntaxTheme     string  // Syntax theme (e.g. monokai, dracula, nord, etc.)
}

// DefaultConfig returns production-ready default settings.
func DefaultConfig() Config {
	return Config{
		FPS:             30,
		Density:         50,
		SpeedScale:      1.0,
		ThemeName:       "green",
		CharSetName:     string(CharSetMatrix),
		BoldHead:        true,
		TabWidth:        4,
		SyntaxHighlight: true,
		SyntaxTheme:     "monokai",
	}
}

// Cell represents one terminal character grid cell.
type Cell struct {
	Rune  rune
	Color string
}

// Engine drives the matrix rain simulation and rendering loop.
type Engine struct {
	cfg              Config
	mu               sync.Mutex
	width            int
	height           int
	columns          []*Column
	pool             []rune
	theme            Theme
	themeIndex       int
	paused           bool
	writer           *bufio.Writer
	stdinFd          int
	stdoutFd         int
	termState        *term.State
	ttyFile          *os.File
	textBuf          *TextBuffer
	targetGrid       [][]TargetCell
	textDrops        []*TextDrop
	scrollOffset     int
	syntaxHighlight  bool
	syntaxThemeIndex int

	// Search state
	searching     bool
	searchQuery   string
	activeQuery   string
	searchMatches []SearchMatch
	matchesByLine map[int][]SearchMatch
	currentMatch  int
	searchStatus  string
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
	if cfg.SyntaxTheme == "" {
		cfg.SyntaxTheme = "monokai"
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
		textBuf = NewTextBuffer(cfg.FileContent, cfg.FilePath, cfg.SyntaxHighlight, cfg.SyntaxTheme, cfg.TabWidth)
	}

	syntaxThemeIdx := 0
	for i, st := range DefaultSyntaxThemes {
		if st == cfg.SyntaxTheme {
			syntaxThemeIdx = i
			break
		}
	}

	return &Engine{
		cfg:              cfg,
		pool:             GetCharPool(CharSet(cfg.CharSetName)),
		theme:            theme,
		themeIndex:       themeIdx,
		writer:           bufio.NewWriterSize(out, 64*1024), // 64KB render buffer
		stdinFd:          int(os.Stdin.Fd()),
		stdoutFd:         int(os.Stdout.Fd()),
		textBuf:          textBuf,
		syntaxHighlight:  cfg.SyntaxHighlight,
		syntaxThemeIndex: syntaxThemeIdx,
		currentMatch:     -1,
		matchesByLine:    make(map[int][]SearchMatch),
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
		if e.activeQuery != "" {
			e.findMatches(e.activeQuery)
		}
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

// textViewHeight returns the screen rows available for text lines, reserving the bottom row for search/status bar.
func (e *Engine) textViewHeight() int {
	if e.isTextMode() && (e.searching || e.activeQuery != "" || e.searchStatus != "") {
		if e.height > 1 {
			return e.height - 1
		}
	}
	return e.height
}

// maxScrollOffset returns the maximum allowed scroll offset given current height.
func (e *Engine) maxScrollOffset() int {
	if e.textBuf == nil {
		return 0
	}
	viewH := e.textViewHeight()
	total := e.textBuf.TotalLines(e.width)
	if total <= viewH {
		return 0
	}
	return total - viewH
}

// rebuildTargetGrid computes target rune positions for the current visible window.
func (e *Engine) rebuildTargetGrid() {
	if e.textBuf == nil || e.width <= 0 || e.height <= 0 {
		return
	}

	viewH := e.textViewHeight()
	wrapped := e.textBuf.Wrap(e.width)
	numLines := len(wrapped)

	maxOffset := e.maxScrollOffset()
	if e.scrollOffset > maxOffset {
		e.scrollOffset = maxOffset
	}
	if e.scrollOffset < 0 {
		e.scrollOffset = 0
	}

	end := e.scrollOffset + viewH
	if end > numLines {
		end = numLines
	}

	var visible [][]StyledRune
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
		startRow = (viewH - len(visible)) / 2
		if startRow < 0 {
			startRow = 0
		}
		startCol = (e.width - maxLen) / 2
		if startCol < 0 {
			startCol = 0
		}
	}

	target := make([][]TargetCell, e.height)
	for r := range target {
		target[r] = make([]TargetCell, e.width)
	}

	for i, line := range visible {
		r := startRow + i
		if r >= viewH {
			break
		}
		docLine := e.scrollOffset + i
		for j, sr := range line {
			c := startCol + j
			if c >= e.width {
				break
			}
			if sr.Rune != ' ' {
				target[r][c] = TargetCell{
					Rune:    sr.Rune,
					Color:   sr.Color,
					DocLine: docLine,
					Col:     j,
				}
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

// findMatches searches the wrapped text lines for all occurrences of the query.
func (e *Engine) findMatches(query string) {
	e.searchMatches = nil
	e.matchesByLine = make(map[int][]SearchMatch)
	if query == "" || e.textBuf == nil {
		e.currentMatch = -1
		return
	}

	wrapped := e.textBuf.Wrap(e.width)
	queryLower := strings.ToLower(query)
	queryRunes := []rune(queryLower)
	queryLen := len(queryRunes)
	if queryLen == 0 {
		return
	}

	matchIdx := 0
	for lineIdx, line := range wrapped {
		lineRunes := make([]rune, len(line))
		for i, sr := range line {
			lineRunes[i] = sr.Rune
		}
		lineLower := strings.ToLower(string(lineRunes))
		lineLowerRunes := []rune(lineLower)

		for i := 0; i <= len(lineLowerRunes)-queryLen; i++ {
			matched := true
			for j := 0; j < queryLen; j++ {
				if lineLowerRunes[i+j] != queryRunes[j] {
					matched = false
					break
				}
			}
			if matched {
				m := SearchMatch{
					Index: matchIdx,
					Line:  lineIdx,
					Col:   i,
					Len:   queryLen,
				}
				e.searchMatches = append(e.searchMatches, m)
				e.matchesByLine[lineIdx] = append(e.matchesByLine[lineIdx], m)
				matchIdx++
			}
		}
	}

	if len(e.searchMatches) > 0 {
		if e.currentMatch < 0 || e.currentMatch >= len(e.searchMatches) {
			e.currentMatch = 0
		}
	} else {
		e.currentMatch = -1
	}
}

// isMatch reports whether the cell at (docLine, col) is part of any match, and whether it is the active match.
func (e *Engine) isMatch(docLine int, col int) (bool, bool) {
	lineMatches, ok := e.matchesByLine[docLine]
	if !ok || len(lineMatches) == 0 {
		return false, false
	}
	for _, m := range lineMatches {
		if col >= m.Col && col < m.Col+m.Len {
			return true, m.Index == e.currentMatch
		}
	}
	return false, false
}

// executeSearch initiates a search for the current searchQuery, jumps to the first match, and fast-forwards rain.
func (e *Engine) executeSearch() {
	if e.searchQuery == "" {
		return
	}
	e.activeQuery = e.searchQuery
	e.findMatches(e.activeQuery)

	if len(e.searchMatches) > 0 {
		e.currentMatch = 0
		e.searchStatus = ""
		e.fastForwardTextRain()
		e.scrollToMatch(e.searchMatches[0])
	} else {
		e.currentMatch = -1
		e.searchStatus = fmt.Sprintf("Pattern not found: %s", e.activeQuery)
		e.rebuildTargetGrid()
	}
}

// scrollToMatch scrolls the viewport so the specified match is visible with context.
func (e *Engine) scrollToMatch(match SearchMatch) {
	viewH := e.textViewHeight()
	desired := match.Line - viewH/3
	if desired < 0 {
		desired = 0
	}
	maxOffset := e.maxScrollOffset()
	if desired > maxOffset {
		desired = maxOffset
	}
	e.scrollOffset = desired
	e.rebuildTargetGrid()
}

// nextMatch jumps to the next search occurrence, wrapping around if at the end.
func (e *Engine) nextMatch() {
	if len(e.searchMatches) == 0 {
		if e.activeQuery != "" {
			e.searchStatus = fmt.Sprintf("Pattern not found: %s", e.activeQuery)
			e.rebuildTargetGrid()
		}
		return
	}
	e.currentMatch = (e.currentMatch + 1) % len(e.searchMatches)
	e.searchStatus = ""
	e.fastForwardTextRain()
	e.scrollToMatch(e.searchMatches[e.currentMatch])
}

// prevMatch jumps to the previous search occurrence, wrapping around if at the start.
func (e *Engine) prevMatch() {
	if len(e.searchMatches) == 0 {
		if e.activeQuery != "" {
			e.searchStatus = fmt.Sprintf("Pattern not found: %s", e.activeQuery)
			e.rebuildTargetGrid()
		}
		return
	}
	e.currentMatch = (e.currentMatch - 1 + len(e.searchMatches)) % len(e.searchMatches)
	e.searchStatus = ""
	e.fastForwardTextRain()
	e.scrollToMatch(e.searchMatches[e.currentMatch])
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

	viewH := e.textViewHeight()

	for x := 0; x < e.width; x++ {
		var drop *TextDrop
		if x < len(e.textDrops) {
			drop = e.textDrops[x]
		}

		for y := 0; y < viewH; y++ {
			var target TargetCell
			if y < len(e.targetGrid) && x < len(e.targetGrid[y]) {
				target = e.targetGrid[y][x]
			}

			// Determine settled color: search highlight takes precedence, then syntax, then theme
			settledColor := ""
			if target.Rune != 0 {
				isMatched, isActive := e.isMatch(target.DocLine, target.Col)
				if isMatched {
					if isActive {
						settledColor = "\x1b[1;30;48;2;255;235;59m" // Bold black on gold (active match)
					} else {
						settledColor = "\x1b[1;30;48;2;255;160;0m"  // Bold black on amber (other match)
					}
				} else if e.syntaxHighlight && target.Color != "" {
					settledColor = target.Color
				} else {
					settledColor = e.theme.SettledColor(x)
				}
			}

			if drop == nil || drop.Done {
				// Settled state for this cell
				if target.Rune != 0 {
					grid[y][x] = Cell{
						Rune:  target.Rune,
						Color: settledColor,
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
				r := target.Rune
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
				r := target.Rune
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
				if target.Rune != 0 {
					grid[y][x] = Cell{
						Rune:  target.Rune,
						Color: settledColor,
					}
				}
			}
		}
	}

	// Render bottom search / status bar if active
	if viewH < e.height {
		bottomRow := e.height - 1
		var promptStr string
		var promptColor string

		if e.searching {
			promptStr = "/" + e.searchQuery + "_"
			promptColor = "\x1b[1;37;48;2;35;35;35m" // Bold white on dark charcoal
		} else if e.searchStatus != "" {
			promptStr = e.searchStatus
			promptColor = "\x1b[1;37;48;2;160;30;30m" // Bold white on dark crimson
		} else if e.activeQuery != "" {
			if len(e.searchMatches) > 0 {
				promptStr = fmt.Sprintf("[%d/%d] /%s  (n: next, p: prev, /: search, Esc: clear)",
					e.currentMatch+1, len(e.searchMatches), e.activeQuery)
				promptColor = "\x1b[1;30;48;2;210;210;210m" // Crisp dark text on silver
			} else {
				promptStr = fmt.Sprintf("Pattern not found: %s", e.activeQuery)
				promptColor = "\x1b[1;37;48;2;160;30;30m"
			}
		}

		promptRunes := []rune(promptStr)
		for x := 0; x < e.width; x++ {
			r := ' '
			if x < len(promptRunes) {
				r = promptRunes[x]
			}
			grid[bottomRow][x] = Cell{
				Rune:  r,
				Color: promptColor,
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
			if cell.Rune == 0 || (cell.Rune == ' ' && cell.Color == "") {
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

	// If currently typing in search prompt
	if e.searching {
		switch key {
		case 27, 3: // ESC or Ctrl+C cancels search prompt
			e.searching = false
			e.searchQuery = ""
			e.rebuildTargetGrid()
			return false

		case '\r', '\n': // Enter confirms search
			e.searching = false
			e.executeSearch()
			return false

		case 127, 8: // Backspace
			runes := []rune(e.searchQuery)
			if len(runes) > 0 {
				e.searchQuery = string(runes[:len(runes)-1])
			}
			return false

		default:
			if key >= 32 && key < 1000 {
				e.searchQuery += string(rune(key))
			}
			return false
		}
	}

	// Normal navigation mode
	switch key {
	case 'q', 'Q', 3: // 'q', 'Q', Ctrl+C exits
		return true

	case 27: // ESC key
		// If search is active, ESC clears search highlights; otherwise ESC exits
		if e.isTextMode() && (e.activeQuery != "" || e.searchStatus != "") {
			e.activeQuery = ""
			e.searchMatches = nil
			e.matchesByLine = nil
			e.currentMatch = -1
			e.searchStatus = ""
			e.rebuildTargetGrid()
			return false
		}
		return true

	case '/': // Enter search mode
		if e.isTextMode() {
			e.searching = true
			e.searchQuery = ""
			e.searchStatus = ""
			e.rebuildTargetGrid()
		}

	case 'n': // Next search match
		if e.isTextMode() {
			e.nextMatch()
		}

	case 'p', 'N': // Previous search match
		if e.isTextMode() {
			e.prevMatch()
		}

	case '\r', '\n': // Enter key in normal mode
		if e.isTextMode() {
			if !e.allTextDropsDone() {
				e.fastForwardTextRain()
			}
		}

	case ' ': // Pause / resume
		e.paused = !e.paused

	case 'c', 'C': // Cycle Matrix theme palette
		e.themeIndex = (e.themeIndex + 1) % len(AllThemes)
		e.theme = AllThemes[e.themeIndex]

	case 's', 'S': // Toggle syntax highlighting
		if e.isTextMode() {
			e.syntaxHighlight = !e.syntaxHighlight
			if e.textBuf != nil {
				e.textBuf.SetSyntax(e.syntaxHighlight, e.cfg.SyntaxTheme)
				e.rebuildTargetGrid()
			}
		}

	case 't', 'T': // Cycle syntax theme
		if e.isTextMode() {
			e.syntaxThemeIndex = (e.syntaxThemeIndex + 1) % len(DefaultSyntaxThemes)
			e.cfg.SyntaxTheme = DefaultSyntaxThemes[e.syntaxThemeIndex]
			if e.textBuf != nil {
				e.textBuf.SetSyntax(e.syntaxHighlight, e.cfg.SyntaxTheme)
				e.rebuildTargetGrid()
			}
		}

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
			e.scrollDown(e.textViewHeight() / 2)
		}

	case 'u', 'U', keyPageUp:
		if e.isTextMode() {
			e.scrollUp(e.textViewHeight() / 2)
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
