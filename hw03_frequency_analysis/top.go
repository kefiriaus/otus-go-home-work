package hw03frequencyanalysis

import (
	"slices"
	"strings"
	"unicode"
)

type pair struct {
	key   string
	value int
}

func isOnlyHyphens(s string) bool {
	return len(s) > 1 && strings.TrimRight(s, "-") == ""
}

func isOnlyPunct(s string) bool {
	if s == "-" { return false }
	return len(s) > 0 && strings.TrimFunc(s, unicode.IsPunct) == ""
}

func trimNonLetter(s string) string {
	if isOnlyHyphens(s) {
		return s
	}
	if isOnlyPunct(s) {
		return s
	}
	return strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
}

func Top10(s string) []string {
	ss := strings.Fields(strings.ToLower(s))
	ws := make(map[string]int, len(ss))
	for _, w := range ss {
		w = trimNonLetter(w)
		if w == "" {
			continue
		}
		ws[w]++
	}
	pairs := make([]pair, 0, len(ws))
	for k, v := range ws {
		pairs = append(pairs, pair{key: k, value: v})
	}
	slices.SortFunc(pairs, func(a, b pair) int {
		if a.value == b.value {
			return strings.Compare(a.key, b.key)
		}
		return b.value - a.value
	})

	top := make([]string, 0, 10)
	for i, p := range pairs {
		if i >= 10 {
			break
		}
		top = append(top, p.key)
	}

	return top
}
