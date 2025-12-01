//go:build !unix

package build

func setUmask(mask int) {
}
