//go:build !windows

package sandbox

import "Marcus/internal/model"

// assignToJob is a no-op on non-Windows platforms; there is no Job Object
// equivalent to return.
func assignToJob(pid int, limits model.ResourceLimits) (uintptr, error) {
	return 0, nil
}
