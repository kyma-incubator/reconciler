package file

import "os"

func Exists(file string) bool {
	if file == "" {
		return false
	}
	stats, err := os.Stat(file)
	return !os.IsNotExist(err) && (stats != nil && !stats.IsDir())
}

func DirExists(file string) bool {
	if file == "" {
		return false
	}
	stats, err := os.Stat(file)
	return !os.IsNotExist(err) && (stats != nil && stats.IsDir())
}
