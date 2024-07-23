package utils

import "os"

func IsFileExist(filepath string) bool {
	if _, err := os.Stat(filepath); err != nil {
		return false
	}

	return true
}
