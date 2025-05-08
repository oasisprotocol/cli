//go:build !unix

package build

import (
	"archive/tar"
	"os"
	"time"
)

func extractHandleSpecialNode(path string, header *tar.Header) error {
	return nil
}

func extractChtimes(path string, atime, mtime time.Time) error {
	return os.Chtimes(path, atime, mtime)
}

func setUmask(mask int) {
}
