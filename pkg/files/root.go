package file

import (
	"path/filepath"
	"runtime"
)

var (
	_, b, _, _ = runtime.Caller(0)
	// Root folder of this project, needs to be changed if root.go (this file) is moved relative to the root folder
	Root = filepath.Join(filepath.Dir(b), "../..")
)
