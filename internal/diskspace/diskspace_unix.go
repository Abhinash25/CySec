//go:build !windows
// +build !windows

package diskspace

import (
	"syscall"
)

// getAvailableSpace returns the available disk space in bytes for the given path on Unix.
func getAvailableSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}
	
	// Available blocks * size per block = available space in bytes
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}
