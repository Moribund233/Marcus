//go:build windows

package sandbox

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObject     = kernel32.NewProc("CreateJobObjectW")
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

const (
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x2000
	JOB_OBJECT_LIMIT_ACTIVE_PROCESS    = 0x0008

	PROCESS_SET_QUOTA    = 0x0100
	PROCESS_TERMINATE    = 0x0001
)

func assignToJob(pid int) error {
	job, _, _ := procCreateJobObject.Call(0, 0)
	if job == 0 {
		return fmt.Errorf("CreateJobObject failed")
	}

	info := JOBOBJECT_BASIC_LIMIT_INFORMATION{
		LimitFlags: JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | JOB_OBJECT_LIMIT_ACTIVE_PROCESS,
		ActiveProcessLimit: 1,
	}

	ret, _, _ := procSetInformationJob.Call(
		job,
		2, // JobObjectBasicLimitInformation
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	if ret == 0 {
		procCloseHandle.Call(job)
		return fmt.Errorf("SetInformationJobObject failed")
	}

	handle, err := syscall.OpenProcess(PROCESS_SET_QUOTA|PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		procCloseHandle.Call(job)
		return fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}

	ret, _, _ = procAssignProcessToJob.Call(job, uintptr(handle))
	syscall.CloseHandle(handle)
	if ret == 0 {
		procCloseHandle.Call(job)
		return fmt.Errorf("AssignProcessToJobObject failed")
	}

	return nil
}
