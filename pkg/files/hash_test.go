package file

import (
	"encoding/base64"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func h(s string, hasher func() hash.Hash) string {
	h := hasher()
	_, _ = io.Copy(h, strings.NewReader(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func htop(s string, hasher func() hash.Hash) string {
	h := hasher()
	_, _ = io.Copy(h, strings.NewReader(s))
	return "hfnv:" + base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func TestHashFnv(t *testing.T) {
	hasher := func() hash.Hash { return fnv.New128a() }
	files := []string{"xyz", "abc"}
	open := func(name string) (io.ReadCloser, error) {
		return ioutil.NopCloser(strings.NewReader(fmt.Sprintf("data for %s", name))), nil
	}
	expected := fmt.Sprintf("%s  %s\n%s  %s\n",
		h("data for abc", hasher), "abc",
		h("data for xyz", hasher), "xyz",
	)
	want := htop(expected, hasher)
	out, err := HashFnv("")(files, open)
	if err != nil {
		t.Fatal(err)
	}
	if out != want {
		t.Errorf("HashFnv(...) = %s, want %s", out, want)
	}

	_, err = HashFnv("")([]string{"xyz", "a\nbc"}, open)
	if err == nil {
		t.Error("HashFnv: expected error on newline in filenames")
	}
}

func TestHashDir(t *testing.T) {
	hasher := func() hash.Hash { return fnv.New128a() }
	dir, err := ioutil.TempDir("", "dirhash-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer func(path string) { _ = os.RemoveAll(path) }(dir)
	if err := ioutil.WriteFile(filepath.Join(dir, "xyz"), []byte("data for xyz"), 0666); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "abc"), []byte("data for abc"), 0666); err != nil {
		t.Fatal(err)
	}
	want := htop(fmt.Sprintf("%s  %s\n%s  %s\n", h("data for abc", hasher), "prefix/abc", h("data for xyz", hasher), "prefix/xyz"), hasher)
	out, err := HashDir(dir, "prefix", HashFnv(""))
	if err != nil {
		t.Fatalf("HashDir: %v", err)
	}
	if out != want {
		t.Errorf("HashDir(...) = %s, want %s", out, want)
	}
}

func TestDirFiles(t *testing.T) {
	dir, err := ioutil.TempDir("", "dirfiles-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(dir)
	if err := ioutil.WriteFile(filepath.Join(dir, "xyz"), []byte("data for xyz"), 0666); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "abc"), []byte("data for abc"), 0666); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0777); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "subdir", "xyz"), []byte("data for subdir xyz"), 0666); err != nil {
		t.Fatal(err)
	}
	prefix := "foo/bar@v2.3.4"
	out, err := DirFiles(dir, prefix)
	if err != nil {
		t.Fatalf("DirFiles: %v", err)
	}
	for _, file := range out {
		if !strings.HasPrefix(file, prefix) {
			t.Errorf("Dir file = %s, want prefix %s", file, prefix)
		}
	}
}
