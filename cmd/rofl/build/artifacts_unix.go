//go:build unix

package build

import (
	"archive/tar"
	"time"

	"golang.org/x/sys/unix"
)

func extractHandleSpecialNode(path string, header *tar.Header) error {
	mode := uint32(header.Mode & 0o7777)
	switch header.Typeflag {
	case tar.TypeBlock:
		mode |= unix.S_IFBLK
	case tar.TypeChar:
		mode |= unix.S_IFCHR
	case tar.TypeFifo:
		mode |= unix.S_IFIFO
	}

	return unix.Mknod(path, mode, int(unix.Mkdev(uint32(header.Devmajor), uint32(header.Devminor))))
}

func extractChtimes(path string, atime, mtime time.Time) error {
	atv := unix.NsecToTimeval(atime.UnixNano())
	mtv := unix.NsecToTimeval(mtime.UnixNano())
	return unix.Lutimes(path, []unix.Timeval{atv, mtv})
}
