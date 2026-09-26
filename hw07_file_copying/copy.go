package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	ErrUnsupportedFile       = errors.New("unsupported file")
	ErrOffsetExceedsFileSize = errors.New("offset exceeds file size")
	ErrNegativeArgs          = errors.New("offset and limit must be non-negative")
	ErrSameFile              = errors.New("source and destination are the same file")
)

func Copy(fromPath, toPath string, offset, limit int64) error {
	return copyFile(fromPath, toPath, offset, limit, os.Stderr)
}

func copyFile(fromPath, toPath string, offset, limit int64, progressOut io.Writer) (err error) {
	if offset < 0 || limit < 0 {
		return ErrNegativeArgs
	}

	src, err := os.Open(filepath.Clean(fromPath))
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer func() { _ = src.Close() }()

	srcInfo, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}
	if !srcInfo.Mode().IsRegular() {
		return ErrUnsupportedFile
	}

	size := srcInfo.Size()
	if offset > size {
		return ErrOffsetExceedsFileSize
	}

	toCopy := size - offset
	if limit > 0 && limit < toCopy {
		toCopy = limit
	}

	if dstInfo, statErr := os.Stat(toPath); statErr == nil && os.SameFile(srcInfo, dstInfo) {
		return ErrSameFile
	}

	if _, err = src.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek source: %w", err)
	}

	dst, err := os.Create(filepath.Clean(toPath))
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer func() {
		if cerr := dst.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close destination: %w", cerr)
		}
	}()

	bar := newProgress(progressOut, toCopy)
	_, err = io.CopyN(dst, io.TeeReader(src, bar), toCopy)
	bar.finish()
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return nil
}
