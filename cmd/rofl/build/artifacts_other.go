//go:build !unix

package build //revive:disable

func setUmask(mask int) {
}
