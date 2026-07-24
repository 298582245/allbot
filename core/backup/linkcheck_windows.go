//go:build windows

package backup

import (
	"os"
	"syscall"
)

const fileAttributeReparsePoint = 0x400

func hasReparsePoint(info os.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&fileAttributeReparsePoint != 0
}
