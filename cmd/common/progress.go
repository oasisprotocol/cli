package common

import (
	"fmt"
	"io"
	"os"
	"time"
)

// ProgressOption configures the behaviour of ProgressWriter.
type ProgressOption func(*ProgressWriter)

// WithMessage sets the status prefix shown before percentage/KiB.
func WithMessage(msg string) ProgressOption {
	return func(p *ProgressWriter) { p.msg = msg }
}

// WithFrequency sets how often updates are printed.
func WithFrequency(freq time.Duration) ProgressOption {
	return func(p *ProgressWriter) { p.freq = freq }
}

// WithOutput sets the destination writer (defaults to os.Stderr).
func WithOutput(w io.Writer) ProgressOption {
	return func(p *ProgressWriter) { p.out = w }
}

// ProgressWriter is an io.Writer that prints a simple progress bar.
// It is safe to use with io.TeeReader.
//
//	pw := common.NewProgressWriter(resp.ContentLength, common.WithMessage("Downloading..."))
//	io.Copy(dst, io.TeeReader(src, pw))
type ProgressWriter struct {
	written    int64
	lastPrint  time.Time
	contentLen int64

	msg  string
	freq time.Duration
	out  io.Writer
}

const defaultFreq = 500 * time.Millisecond

// NewProgressWriter returns a configured ProgressWriter.
func NewProgressWriter(contentLen int64, opts ...ProgressOption) *ProgressWriter {
	pw := &ProgressWriter{
		contentLen: contentLen,
		msg:        "Progress...",
		freq:       defaultFreq,
		out:        os.Stderr,
	}
	for _, o := range opts {
		o(pw)
	}
	return pw
}

// Write implements io.Writer.
func (p *ProgressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.written += int64(n)
	now := time.Now()

	if now.Sub(p.lastPrint) >= p.freq || p.written == p.contentLen {
		if p.contentLen > 0 {
			percent := float64(p.written) / float64(p.contentLen) * 100
			fmt.Fprintf(p.out, "\r%s %5.1f%%", p.msg, percent)
			if p.written == p.contentLen {
				fmt.Fprintln(p.out)
			}
		} else {
			fmt.Fprintf(p.out, "\r%s %d KiB", p.msg, p.written/1024)
		}
		p.lastPrint = now
	}
	return n, nil
}
