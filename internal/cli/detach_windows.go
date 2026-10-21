//go:build windows

package cli

import "syscall"

const (
	createNewProcessGroup = 0x00000200
	detachedProcessFlag   = 0x00000008
)

// detachedProcess starts a child without a console, so it outlives the
// terminal that started it.
func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | detachedProcessFlag, HideWindow: true}
}
