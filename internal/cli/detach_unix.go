//go:build !windows

package cli

import "syscall"

// detachedProcess starts a child in its own session, so it outlives the
// terminal that started it.
func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
