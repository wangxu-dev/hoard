//go:build !darwin

package app

// Other platforms retain their existing native frame behavior.
func setNativeTitlebar(handle uintptr, dark bool, r, g, b uint8) {}
