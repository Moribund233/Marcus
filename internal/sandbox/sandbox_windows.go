//go:build windows

package sandbox

import (
	"fmt"
	"log"
	"syscall"
	"time"
	"unsafe"

	"Marcus/internal/model"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObject    = kernel32.NewProc("CreateJobObjectW")
	procAssignProcessToJob = kernel32.NewProc("AssignProcessToJobObject")
	procSetInformationJob  = kernel32.NewProc("SetInformationJobObject")
	procTerminateJobObject = kernel32.NewProc("TerminateJobObject")
	procCloseHandle        = kernel32.NewProc("CloseHandle")
)

type JOBOBJECT_BASIC_LIMIT_INFORMATION struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	ChildProcessRate        uint32
	ExtendedLimitInfoFlags  uint32
}

type JOBOBJECT_EXTENDED_LIMIT_INFORMATION struct {
	BasicLimitInformation JOBOBJECT_BASIC_LIMIT_INFORMATION
	IoInfo                [3]uint64
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

const (
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x2000
	JOB_OBJECT_LIMIT_ACTIVE_PROCESS    = 0x0008
	JOB_OBJECT_LIMIT_JOB_MEMORY        = 0x0200
	JOB_OBJECT_LIMIT_JOB_TIME          = 0x0004

	JobObjectBasicLimitInformation    = 2
	JobObjectExtendedLimitInformation = 9

	PROCESS_SET_QUOTA = 0x0100
	PROCESS_TERMINATE = 0x0001
)

// assignToJob creates a Windows Job Object, applies the configured limits, and
// assigns the given process to it. The returned handle must be closed by the
// caller; doing so terminates all processes in the job when
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE is set.
//
// If the caller is already running inside a job (common for IDE/test runners),
// KILL_ON_JOB_CLOSE may be rejected. In that case we fall back to
// ACTIVE_PROCESS at least, and rely on the caller's Shutdown/Stop logic to
// explicitly kill the direct child process.
func assignToJob(pid int, limits model.ResourceLimits) (uintptr, error) {
	job, _, _ := procCreateJobObject.Call(0, 0)
	if job == 0 {
		return 0, fmt.Errorf("CreateJobObject failed")
	}

	basic := JOBOBJECT_BASIC_LIMIT_INFORMATION{}
	if limits.TimeoutSeconds > 0 {
		basic.LimitFlags |= JOB_OBJECT_LIMIT_JOB_TIME
		basic.PerJobUserTimeLimit = int64(time.Duration(limits.TimeoutSeconds) * time.Second / 100)
	}

	configured := false
	if limits.MemoryLimitMB > 0 {
		basic.LimitFlags |= JOB_OBJECT_LIMIT_JOB_MEMORY
		ext := JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: basic,
			JobMemoryLimit:        uintptr(limits.MemoryLimitMB) * 1024 * 1024,
		}
		if ret, _, _ := procSetInformationJob.Call(
			job,
			JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&ext)),
			uintptr(uint32(unsafe.Sizeof(ext))),
		); ret != 0 {
			configured = true
		} else {
			log.Printf("sandbox: extended job limits not accepted, will retry basic limits")
		}
	}

	if !configured {
			// KILL_ON_JOB_CLOSE ensures the whole child process tree is terminated
			// when the job handle is closed on Stop/Shutdown. We intentionally do
			// NOT set ACTIVE_PROCESS here: tools like uv trampoline spawn a Python
			// child process to do actual work, and ActiveProcessLimit=1 would
			// prevent that with ERROR_NOT_ENOUGH_QUOTA (os error 1816).
			basic.LimitFlags |= JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

			ret, _, _ := procSetInformationJob.Call(
				job,
				JobObjectBasicLimitInformation,
				uintptr(unsafe.Pointer(&basic)),
				uintptr(uint32(unsafe.Sizeof(basic))),
			)
			if ret == 0 {
				// KILL_ON_JOB_CLOSE is often disallowed when the current process is
				// already in a job (e.g. IDE/terminal). In that case we rely on the
				// caller's Stop/Shutdown logic to explicitly kill the direct child via
				// cmd.Process.Kill() — no job-level cleanup available.
				procCloseHandle.Call(job)
				return 0, fmt.Errorf("KILL_ON_JOB_CLOSE unavailable (nested job?) — falling back to direct kill")
			}
		}

	handle, err := syscall.OpenProcess(PROCESS_SET_QUOTA|PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		procCloseHandle.Call(job)
		return 0, fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}

	ret, _, _ := procAssignProcessToJob.Call(job, uintptr(handle))
	syscall.CloseHandle(handle)
	if ret == 0 {
		procCloseHandle.Call(job)
		return 0, fmt.Errorf("AssignProcessToJobObject failed")
	}

	return job, nil
}
