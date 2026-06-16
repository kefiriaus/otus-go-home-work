package hw02unpackstring

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
)

const EscapeRune = '\\'

var ErrInvalidString = errors.New("invalid string")

type Rune struct {
	r       rune
	escaped bool
}

func (p *Rune) Update(r rune, escaped bool) {
	p.r = r
	p.escaped = escaped
}

func (p Rune) IsEmpty() bool {
	return p.r == 0
}

func (p Rune) IsLetter() bool {
	return p.escaped || unicode.IsLetter(p.r) || unicode.IsSpace(p.r)
}

func (p Rune) IsDigit() bool {
	return !p.escaped && unicode.IsDigit(p.r)
}

func (p Rune) IsEscape() bool {
	return !p.escaped && p.r == EscapeRune
}

func (p Rune) IsEscaped() bool {
	return p.escaped
}

func UnpackRune(prev, current Rune, isCurrentLast bool) (res []rune, escaped bool, err error) {
	if prev.IsEmpty() && current.IsDigit() {
		return nil, false, ErrInvalidString
	}
	if prev.IsEmpty() && current.IsEscape() {
		return nil, false, ErrInvalidString
	}
	if prev.IsDigit() && current.IsDigit() {
		return nil, false, ErrInvalidString
	}
	if prev.IsEscape() && current.IsLetter() {
		return nil, false, ErrInvalidString
	}

	switch {
	case prev.IsEscape() && !current.IsEmpty():
		res = append(res, current.r)
		return res, true, nil
	case prev.IsLetter() && !prev.IsEscaped() && !current.IsDigit():
		res = append(res, prev.r)
	case prev.IsLetter() && current.IsDigit():
		n, err := strconv.Atoi(string(current.r))
		if err != nil {
			return nil, false, err
		}
		if n > 0 {
			if prev.IsEscaped() {
				n--
			}
			res = []rune(strings.Repeat(string(prev.r), n))
		}
	}

	if isCurrentLast && current.IsLetter() {
		res = append(res, current.r)
	}

	return res, escaped, nil
}

func Unpack(s string) (string, error) {
	sLen := len(s)
	if sLen == 0 {
		return "", nil
	}

	var prev, current Rune
	var isCurrentLast, escaped bool
	var res []rune
	var err error
	ss := strings.Builder{}
	for i, r := range s {
		current.Update(r, false)
		isCurrentLast = i == sLen-1
		res, escaped, err = UnpackRune(prev, current, isCurrentLast)
		if err != nil {
			return "", err
		}
		prev.Update(r, escaped)
		ss.WriteString(string(res))
	}
	return ss.String(), nil
}
