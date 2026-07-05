//go:build !windows

package sandbox

// closeJobHandle is a no-op on non-Windows platforms because Job Objects are
// a Windows-specific mechanism.
func closeJobHandle(h uintptr) {}
