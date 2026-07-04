//go:build !windows

package sandbox

func assignToJob(pid int) error {
	return nil
}
