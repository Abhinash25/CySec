package diskspace

import (
	"golang.org/x/sys/windows"
)

// getAvailableSpace returns the available disk space in bytes for the given path on Windows.
func getAvailableSpace(path string) (int64, error) {
	var freeBytesAvailableToCaller, totalNumberOfBytes, totalNumberOfFreeBytes uint64

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	err = windows.GetDiskFreeSpaceEx(pathPtr,
		&freeBytesAvailableToCaller,
		&totalNumberOfBytes,
		&totalNumberOfFreeBytes)
	
	if err != nil {
		return 0, err
	}

	return int64(freeBytesAvailableToCaller), nil
}
