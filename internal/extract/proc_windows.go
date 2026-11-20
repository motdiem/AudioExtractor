package extract

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// hideWindow stops a console window flashing up when a GUI app starts ffmpeg.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

var (
	jobOnce sync.Once
	job     windows.Handle // never closed: it closes when we exit, however we exit
)

// killWithUs puts ffmpeg in a Job Object that kills its processes when its
// last handle closes. We hold that handle until we exit, so ffmpeg dies with
// the app even when the app is killed outright (Task Manager, a crash), where
// ServiceShutdown never runs. Best effort: on any error ffmpeg simply runs
// unattached, as before.
func killWithUs(p *os.Process) {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			windows.CloseHandle(h)
			return
		}
		job = h
	})
	if job == 0 || p == nil {
		return
	}
	ph, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(ph)
	_ = windows.AssignProcessToJobObject(job, ph)
}

// setHidden sets or clears FILE_ATTRIBUTE_HIDDEN. The leading dot of
// ".audioextractor-*.part" hides the temporary file on macOS only; Explorer
// needs the attribute.
func setHidden(path string, hidden bool) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return err
	}
	if hidden {
		attrs |= windows.FILE_ATTRIBUTE_HIDDEN
	} else {
		attrs &^= windows.FILE_ATTRIBUTE_HIDDEN
	}
	if attrs == 0 {
		attrs = windows.FILE_ATTRIBUTE_NORMAL
	}
	return windows.SetFileAttributes(p, attrs)
}
