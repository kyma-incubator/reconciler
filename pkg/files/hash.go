package file

import (
	"encoding/base64"
	"fmt"
	"github.com/pkg/errors"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A Hash is a directory hash function.
// It accepts a list of files along with a function that opens the content of each file.
// It opens, reads, hashes, and closes each file and returns the overall directory hash.
type Hash func(files []string, open func(string) (io.ReadCloser, error)) (string, error)

// DirFiles returns the list of files in the tree rooted at dir,
// replacing the directory name dir with prefix in each name.
// The resulting names always use forward slashes.
func DirFiles(dir, prefix string) ([]string, error) {
	var files []string
	dir = filepath.Clean(dir)
	err := filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel := file
		if dir != "." {
			rel = file[len(dir)+1:]
		}
		f := filepath.Join(prefix, rel)
		files = append(files, filepath.ToSlash(f))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// HashDir returns the hash of the local file system directory dir,
// replacing the directory name itself with prefix in the file names
// used in the hash function.
func HashDir(dir, prefix string, hash Hash) (string, error) {
	files, err := DirFiles(dir, prefix)
	if err != nil {
		return "", err
	}
	osOpen := func(name string) (io.ReadCloser, error) {
		return os.Open(filepath.Join(dir, strings.TrimPrefix(name, prefix)))
	}
	return hash(files, osOpen)
}

// HashFnv is the "hfnv:" directory hash function, using any algorithm.
//
// HashFnv is "hfnv:" followed by the base64-encoded hash of all given files with the allowed suffix
// specified.
//
// More precisely, the hashed summary contains a single line for each file in the list,
// ordered by sort.Strings applied to the file names, where each line consists of
// the hash of the file content,
// two spaces (U+0020), the file name, and a newline (U+000A).
//
// File names with newlines (U+000A) are disallowed.
func HashFnv(allowedSuffix string) Hash {
	return func(files []string, open func(string) (io.ReadCloser, error)) (string, error) {
		h := fnv.New128a()
		files = append([]string(nil), files...)
		sort.Strings(files)
		for _, file := range files {
			if allowedSuffix != "" && !strings.HasSuffix(file, allowedSuffix) {
				continue
			}
			if strings.Contains(file, "\n") {
				return "", errors.New("dirhash: filenames with newlines are not supported")
			}
			r, err := open(file)
			if err != nil {
				_ = r.Close()
				return "", err
			}
			hf := fnv.New128a()
			_, err = io.Copy(hf, r)
			_ = r.Close()
			if err != nil {
				return "", err
			}
			fmt.Printf("%x  %s\n", hf.Sum(nil), file)
			_, _ = fmt.Fprintf(h, "%x  %s\n", hf.Sum(nil), file)
		}
		return "hfnv:" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
	}
}
