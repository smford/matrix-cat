package matrix

import (
	"crypto/rand"
	"math/big"
)

// Drop represents an individual falling stream of characters within a column.
type Drop struct {
	Y           float64 // Current head position (continuous row coordinate)
	Speed       float64 // Velocity in rows per tick
	Length      int     // Trail length in rows
	Glyphs      []rune  // Buffer of runes in trail: index 0 is head, index Length-1 is tail
	lastHeadRow int     // Tracks integer row transitions
}

// NewDrop creates a new drop configured for the given column height and character pool.
func NewDrop(height int, pool []rune) *Drop {
	// Trail length between 6 and 28 or up to height
	minLen := 6
	maxLen := 28
	if height > 10 && height < maxLen {
		maxLen = height
	}
	length := randomInt(minLen, maxLen)

	// Speed variation: fast drops (0.8 - 1.4 rows/tick), medium (0.4 - 0.8), slow (0.2 - 0.4)
	speedRoll := randomInt(1, 100)
	var speed float64
	switch {
	case speedRoll <= 25:
		// Fast drop
		speed = 0.8 + float64(randomInt(0, 60))/100.0
	case speedRoll <= 75:
		// Medium drop
		speed = 0.4 + float64(randomInt(0, 40))/100.0
	default:
		// Slow drop
		speed = 0.2 + float64(randomInt(0, 20))/100.0
	}

	glyphs := make([]rune, length)
	for i := range glyphs {
		glyphs[i] = RandomRune(pool)
	}

	// Start above the screen so it rains in naturally
	startOffset := float64(randomInt(0, 15))
	initialY := -float64(length) - startOffset

	return &Drop{
		Y:           initialY,
		Speed:       speed,
		Length:      length,
		Glyphs:      glyphs,
		lastHeadRow: int(initialY),
	}
}

// Update advances the drop's position and mutates characters along the trail.
func (d *Drop) Update(pool []rune, speedScale float64) {
	if speedScale <= 0 {
		speedScale = 1.0
	}
	d.Y += d.Speed * speedScale
	currentHeadRow := int(d.Y)

	// If head crossed one or more integer rows, shift glyphs forward
	if currentHeadRow > d.lastHeadRow {
		steps := currentHeadRow - d.lastHeadRow
		for s := 0; s < steps; s++ {
			// Shift glyphs backwards (index 0 is newest head)
			for i := len(d.Glyphs) - 1; i > 0; i-- {
				d.Glyphs[i] = d.Glyphs[i-1]
			}
			d.Glyphs[0] = RandomRune(pool)
		}
		d.lastHeadRow = currentHeadRow
	}

	// Matrix effect: random glyph mutation in the trail (approx 5% chance per glyph)
	for i := 1; i < len(d.Glyphs); i++ {
		if randomInt(1, 100) <= 5 {
			d.Glyphs[i] = RandomRune(pool)
		}
	}
}

// IsOffScreen returns true if the entire tail of the drop has passed below the screen.
func (d *Drop) IsOffScreen(screenHeight int) bool {
	tailRow := int(d.Y) - d.Length
	return tailRow >= screenHeight
}

// Column manages drops in a single vertical column.
type Column struct {
	Drops []*Drop
}

// NewColumn creates an initialized column.
func NewColumn() *Column {
	return &Column{
		Drops: make([]*Drop, 0, 2),
	}
}

// Update updates all drops in this column, removes off-screen drops, and spawns new ones.
func (c *Column) Update(height int, pool []rune, densityPercent int, speedScale float64) {
	active := c.Drops[:0]
	canSpawn := true

	for _, d := range c.Drops {
		d.Update(pool, speedScale)
		if !d.IsOffScreen(height) {
			active = append(active, d)
			// If drop head is still near top, delay spawning next drop
			tailRow := int(d.Y) - d.Length
			if tailRow < 0 {
				canSpawn = false
			}
		}
	}
	c.Drops = active

	// Spawn a new drop if column is clear or tail has cleared top with probability
	if canSpawn && len(c.Drops) < 2 {
		spawnChance := densityPercent / 3 // Scale to frame tick rate
		if spawnChance < 1 {
			spawnChance = 1
		}
		if randomInt(1, 100) <= spawnChance {
			c.Drops = append(c.Drops, NewDrop(height, pool))
		}
	}
}

func randomInt(min, max int) int {
	if min >= max {
		return min
	}
	diff := int64(max - min + 1)
	n, err := rand.Int(rand.Reader, big.NewInt(diff))
	if err != nil {
		return min
	}
	return min + int(n.Int64())
}
