//nolint:revive
package progress

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// PrintProgressBar prints a nice progress bar.
func PrintProgressBar(f io.Writer, msg string, done uint64, total uint64, final bool) {
	// Determine terminal width or fall-back to the standard 80 characters.
	// Note that we need to do this every time in case the user has resized their terminal.
	terminalWidth := 80
	if file, ok := f.(*os.File); ok {
		width, _, err := term.GetSize(int(file.Fd()))
		// Cap the maximum width to 80 to improve readability on wide terminals.
		if err == nil && width < terminalWidth {
			terminalWidth = width
		}
	}

	// Generate a user-friendly string with how many MiB has been transferred so far.
	doneMiB := fmt.Sprintf("%.2f MiB", float64(done)/1024.0/1024.0)

	if total == 0 || done > total {
		// If the total size is unknown, just print how much we've done so far.
		// Also pad to the remainder of the terminal width with spaces, just in case we
		// previously had a progress bar there (so that we erase it).
		out := fmt.Sprintf("%s %s", msg, doneMiB)
		blank := strings.Repeat(" ", terminalWidth-len(out)-1)
		fmt.Fprintf(f, "\r%s%s", out, blank)
	} else {
		// If the total size is known, calculate percentage done and draw progress bar.
		ratioDone := float64(done) / float64(total)
		percentDone := ratioDone * 100.0

		// Status width needed for the message, percentage, and bytes done displays.
		statusWidth := len(msg) + 8 + len(doneMiB)

		if terminalWidth-statusWidth-14 < 0 {
			// Don't draw the progress bar if there's not enough space (where enough space
			// is 14 characters -- 10 for the bar, 2 for the sides, and 2 for the spaces).
			fmt.Fprintf(f, "\r%s %.2f%% %s", msg, percentDone, doneMiB)
		} else {
			// Draw the progress bar.
			availableWidth := terminalWidth - statusWidth - 4
			doneWidth := int(math.Floor(ratioDone * float64(availableWidth)))
			bar := strings.Repeat("#", doneWidth)
			bar += strings.Repeat(" ", availableWidth-doneWidth)

			fmt.Fprintf(f, "\r%s %.2f%% [%s] %s", msg, percentDone, bar, doneMiB)
		}
	}

	if final {
		// If this is the final print, also print a normal newline, so we don't mess up
		// any further output to the terminal.
		fmt.Fprintln(f)
	}
}

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
		PrintProgressBar(p.out, p.msg, uint64(p.written), uint64(p.contentLen), p.contentLen > 0 && p.written == p.contentLen) //nolint:gosec
		p.lastPrint = now
	}
	return n, nil
}
