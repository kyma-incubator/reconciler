package file

import "os"

func Exists(file string) bool {
	stats, err := os.Stat(file)
	return !os.IsNotExist(err) && !stats.IsDir()
}
