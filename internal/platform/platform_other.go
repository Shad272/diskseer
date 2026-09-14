//go:build !windows

package platform

func Detect() Capabilities                { return base() }
func SystemExecutable(name string) string { return "" }
