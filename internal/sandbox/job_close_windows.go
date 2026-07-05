//go:build windows

package sandbox

// closeJobHandle closes a Windows Job Object handle. When the handle is the
// last reference to the job and JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE is set,
// all processes assigned to the job are terminated.
func closeJobHandle(h uintptr) {
	if h != 0 && h != uintptr(^uint(0)) {
		procCloseHandle.Call(h)
	}
}
