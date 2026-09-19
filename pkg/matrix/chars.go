package matrix

import (
	"crypto/rand"
	"math/big"
)

// CharSet defines a set of runes used for the rain effect.
type CharSet string

const (
	CharSetMatrix CharSet = "matrix"
	CharSetASCII  CharSet = "ascii"
	CharSetBinary CharSet = "binary"
	CharSetHex    CharSet = "hex"
)

// matrixRunes contains half-width katakana (single terminal column width) plus digits and symbols.
var matrixRunes = []rune{
	// Half-width Katakana (U+FF66 to U+FF9D) - authentic matrix glyphs
	'ｦ', 'ｧ', 'ｨ', 'ｩ', 'ｪ', 'ｫ', 'ｬ', 'ｭ', 'ｮ', 'ｯ',
	'ｰ', 'ｱ', 'ｲ', 'ｳ', 'ｴ', 'ｵ', 'ｶ', 'ｷ', 'ｸ', 'ｹ',
	'ｺ', 'ｻ', 'ｼ', 'ｽ', 'ｾ', 'ｿ', 'ﾀ', 'ﾁ', 'ﾂ', 'ﾃ',
	'ﾄ', 'ﾅ', 'ﾆ', 'ﾇ', 'ﾈ', 'ﾉ', 'ﾊ', 'ﾋ', 'ﾌ', 'ﾍ',
	'ﾎ', 'ﾏ', 'ﾐ', 'ﾑ', 'ﾒ', 'ﾓ', 'ﾔ', 'ﾕ', 'ﾖ', 'ﾗ',
	'ﾘ', 'ﾙ', 'ﾚ', 'ﾛ', 'ﾜ', 'ﾝ',
	// Numbers & symbols
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	':', '*', '+', '-', '<', '>', '=', '|', '"',
}

var asciiRunes = []rune(
	"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789@#$%&*+-=<>",
)

var binaryRunes = []rune{'0', '1'}

var hexRunes = []rune("0123456789ABCDEF")

// GetCharPool returns the slice of runes corresponding to the requested CharSet.
func GetCharPool(charset CharSet) []rune {
	switch charset {
	case CharSetASCII:
		return asciiRunes
	case CharSetBinary:
		return binaryRunes
	case CharSetHex:
		return hexRunes
	case CharSetMatrix:
		fallthrough
	default:
		return matrixRunes
	}
}

// RandomRune selects a random rune from the given pool using crypto/rand for uniform distribution.
func RandomRune(pool []rune) rune {
	if len(pool) == 0 {
		return ' '
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(pool))))
	if err != nil {
		// Fallback to first rune if entropy source fails
		return pool[0]
	}
	return pool[n.Int64()]
}
