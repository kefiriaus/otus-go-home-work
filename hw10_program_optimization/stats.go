package hw10programoptimization

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

//go:generate easyjson stats.go

//easyjson:json
type User struct {
	Email string
}

type DomainStat map[string]int

func GetDomainStat(r io.Reader, domain string) (DomainStat, error) {
	result := make(DomainStat)
	suffix := "." + domain

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var user User
		if err := user.UnmarshalJSON(line); err != nil {
			return nil, fmt.Errorf("get users error: %w", err)
		}

		if !strings.HasSuffix(user.Email, suffix) {
			continue
		}
		if _, host, ok := strings.Cut(user.Email, "@"); ok {
			result[strings.ToLower(host)]++
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("get users error: %w", err)
	}
	return result, nil
}
