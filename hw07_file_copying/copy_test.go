package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const inputPath = "testdata/input.txt"

func TestCopy(t *testing.T) {
	tests := []struct {
		name          string
		offset, limit int64
		golden        string
	}{
		{"full file", 0, 0, "testdata/out_offset0_limit0.txt"},
		{"limit 10", 0, 10, "testdata/out_offset0_limit10.txt"},
		{"limit 1000", 0, 1000, "testdata/out_offset0_limit1000.txt"},
		{"limit exceeds size", 0, 10000, "testdata/out_offset0_limit10000.txt"},
		{"offset 100 limit 1000", 100, 1000, "testdata/out_offset100_limit1000.txt"},
		{"offset 6000 limit 1000", 6000, 1000, "testdata/out_offset6000_limit1000.txt"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "out.txt")
			var out bytes.Buffer

			require.NoError(t, copyFile(inputPath, dst, tc.offset, tc.limit, &out))
			requireSameContent(t, tc.golden, dst)
			require.True(t, strings.HasSuffix(out.String(), "100%\n"), "progress: %q", out.String())
		})
	}
}

func TestCopyOffsetEqualsSize(t *testing.T) {
	info, err := os.Stat(inputPath)
	require.NoError(t, err)

	dst := filepath.Join(t.TempDir(), "out.txt")
	require.NoError(t, copyFile(inputPath, dst, info.Size(), 0, &bytes.Buffer{}))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestCopyErrors(t *testing.T) {
	info, err := os.Stat(inputPath)
	require.NoError(t, err)

	t.Run("offset exceeds size", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "out.txt")
		err := copyFile(inputPath, dst, info.Size()+1, 0, &bytes.Buffer{})
		require.ErrorIs(t, err, ErrOffsetExceedsFileSize)
	})

	t.Run("negative args", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "out.txt")
		require.ErrorIs(t, copyFile(inputPath, dst, -1, 0, &bytes.Buffer{}), ErrNegativeArgs)
		require.ErrorIs(t, copyFile(inputPath, dst, 0, -1, &bytes.Buffer{}), ErrNegativeArgs)
	})

	t.Run("directory", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "out.txt")
		require.ErrorIs(t, copyFile("testdata", dst, 0, 0, &bytes.Buffer{}), ErrUnsupportedFile)
	})

	t.Run("dev urandom", func(t *testing.T) {
		if _, err := os.Stat("/dev/urandom"); err != nil {
			t.Skip("/dev/urandom not available")
		}
		dst := filepath.Join(t.TempDir(), "out.txt")
		require.ErrorIs(t, copyFile("/dev/urandom", dst, 0, 0, &bytes.Buffer{}), ErrUnsupportedFile)
	})

	t.Run("missing source", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "out.txt")
		require.ErrorIs(t, copyFile("testdata/nope.txt", dst, 0, 0, &bytes.Buffer{}), os.ErrNotExist)
	})

	t.Run("same file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "same.txt")
		require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))

		require.ErrorIs(t, copyFile(path, path, 0, 0, &bytes.Buffer{}), ErrSameFile)

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "data", string(got), "source must stay intact")
	})
}

func requireSameContent(t *testing.T, expectedPath, actualPath string) {
	t.Helper()
	expected, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	actual, err := os.ReadFile(actualPath)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
