package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Environment map[string]EnvValue

// EnvValue helps to distinguish between empty files and files with the first empty line.
type EnvValue struct {
	Value      string
	NeedRemove bool
}

// ReadDir reads a specified directory and returns map of env variables.
// Variables represented as files where filename is name of variable, file first line is a value.
func ReadDir(dir string) (Environment, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read env dir %q: %w", dir, err)
	}

	env := make(Environment, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.Contains(name, "=") {
			continue
		}

		value, err := readValue(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		env[name] = value
	}

	return env, nil
}

func readValue(path string) (EnvValue, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return EnvValue{}, fmt.Errorf("read env file %q: %w", path, err)
	}
	if len(data) == 0 {
		return EnvValue{NeedRemove: true}, nil
	}

	line, _, _ := bytes.Cut(data, []byte("\n"))
	value := strings.TrimRight(string(line), " \t")
	value = strings.ReplaceAll(value, "\x00", "\n")

	return EnvValue{Value: value}, nil
}
