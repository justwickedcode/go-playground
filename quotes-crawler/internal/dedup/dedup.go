package dedup

import (
	"crypto/sha256"
	"fmt"
	"hash/fnv"
	"io"
	"math/bits"
	"regexp"
	"strings"
)

// compiled once at package level
var whitespaceRegex = regexp.MustCompile(`\s+`)

func StripQuoteChars(text string) string {
	return strings.Trim(text, "\"\u201c\u201d«»")
}

func Normalize(text string) string {
	normalizedText := strings.TrimSpace(text)
	normalizedText = StripQuoteChars(normalizedText)
	normalizedText = whitespaceRegex.ReplaceAllString(normalizedText, " ")
	normalizedText = strings.ToLower(normalizedText)
	return normalizedText
}

func SHA256(text string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
}

func Simhash(text string) int64 {
	words := strings.Fields(text)

	// one counter per bit position, int64 to allow negative votes
	var counter [64]int64

	// reuse the same hasher across words to avoid allocations
	h := fnv.New64a()
	for _, word := range words {
		h.Reset()

		// FNV never returns an error but we handle it for interface compliance
		if _, err := io.WriteString(h, word); err != nil {
			return 0
		}

		hash := h.Sum64()

		// each word votes on every bit: +1 if bit is set, -1 if not
		for bit := 0; bit < 64; bit++ {
			if (hash>>bit)&1 == 1 {
				counter[bit]++
			} else {
				counter[bit]--
			}
		}
	}

	// majority vote: if more +1s than -1s, bit is 1 in the fingerprint
	var fingerprint int64
	for bit := 0; bit < 64; bit++ {
		if counter[bit] > 0 {
			fingerprint |= 1 << bit
		}
	}

	return fingerprint
}

func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}
