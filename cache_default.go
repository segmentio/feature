// +build !darwin,!linux

package feature

import (
	"io"
	"os"
)

func mmap(f *os.File) ([]byte, error) {
	s, err := f.Stat()
	if err != nil {
		return nil, err
	}
	b := make([]byte, s.Size())
	_, err = io.ReadFull(f, b)
	return b, err
}

func munmap([]byte) error {
	return nil
}
