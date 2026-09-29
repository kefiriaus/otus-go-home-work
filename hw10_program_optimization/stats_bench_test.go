//go:build bench

package hw10programoptimization

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkGetDomainStat(b *testing.B) {
	zr, err := zip.OpenReader("testdata/users.dat.zip")
	require.NoError(b, err)
	defer zr.Close()

	f, err := zr.File[0].Open()
	require.NoError(b, err)
	data, err := io.ReadAll(f)
	require.NoError(b, err)
	require.NoError(b, f.Close())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := GetDomainStat(bytes.NewReader(data), "biz"); err != nil {
			b.Fatal(err)
		}
	}
}
