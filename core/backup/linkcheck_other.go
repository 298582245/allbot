//go:build !windows

package backup

import "os"

func hasReparsePoint(info os.FileInfo) bool {
	return false
}
