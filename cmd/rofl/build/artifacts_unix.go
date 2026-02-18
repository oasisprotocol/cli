//go:build unix

package build //revive:disable

import (
	"golang.org/x/sys/unix"
)

func setUmask(mask int) {
	unix.Umask(mask)
}
