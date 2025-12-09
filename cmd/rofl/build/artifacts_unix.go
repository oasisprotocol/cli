//go:build unix

package build

import (
	"golang.org/x/sys/unix"
)

func setUmask(mask int) {
	unix.Umask(mask)
}
