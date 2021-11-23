package file

import (
	"bytes"
	"io"
	"os"
)

func ReadFile(source string) ([]byte, error) {
	ioReader, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer ioReader.Close()
	buff := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buff, ioReader)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}
