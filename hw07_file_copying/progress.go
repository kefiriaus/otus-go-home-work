package main

import (
	"fmt"
	"io"
	"strings"
)

type progress struct {
	out     io.Writer
	total   int64
	done    int64
	percent int64
}

const barWidth = 40

func newProgress(out io.Writer, total int64) *progress {
	return &progress{out: out, total: total, percent: -1}
}

func (p *progress) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	p.render()
	return len(b), nil
}

func (p *progress) render() {
	pct := int64(100)
	if p.total > 0 {
		pct = p.done * 100 / p.total
	}
	if pct == p.percent {
		return
	}
	p.percent = pct

	filled := int(pct * barWidth / 100)
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", barWidth-filled)
	_, _ = fmt.Fprintf(p.out, "\r[%s] %3d%%", bar, pct)
}

func (p *progress) finish() {
	p.render()
	_, _ = fmt.Fprintln(p.out)
}
