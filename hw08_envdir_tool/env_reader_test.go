package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadDir(t *testing.T) {
	t.Run("testdata", func(t *testing.T) {
		env, err := ReadDir("testdata/env")
		require.NoError(t, err)
		require.Equal(t, Environment{
			"BAR":   {Value: "bar"},
			"EMPTY": {Value: ""},
			"FOO":   {Value: "   foo\nwith new line"},
			"HELLO": {Value: `"hello"`},
			"UNSET": {NeedRemove: true},
		}, env)
	})

	t.Run("edge cases", func(t *testing.T) {
		dir := t.TempDir() // удаляется автоматически после теста
		writeFile(t, dir, "TRAILING", "value \t \t\nsecond line")
		writeFile(t, dir, "ONLY_NEWLINE", "\n")
		writeFile(t, dir, "ZERO_SIZE", "")
		writeFile(t, dir, "BAD=NAME", "ignored")
		require.NoError(t, os.Mkdir(filepath.Join(dir, "SUBDIR"), 0o755))

		env, err := ReadDir(dir)
		require.NoError(t, err)
		require.Equal(t, Environment{
			"TRAILING":     {Value: "value"},
			"ONLY_NEWLINE": {Value: ""},
			"ZERO_SIZE":    {NeedRemove: true},
		}, env)
	})

	t.Run("missing dir", func(t *testing.T) {
		_, err := ReadDir(filepath.Join(t.TempDir(), "nope"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}
