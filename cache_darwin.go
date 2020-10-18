package feature

import (
	"os"
	"syscall"
)

func mmap(f *os.File) ([]byte, error) {
	s, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fd, size := int(f.Fd()), int(s.Size())
	if size == 0 {
		return nil, nil
	}
	return syscall.Mmap(fd, 0, size, syscall.PROT_READ, syscall.MAP_SHARED)
}

func munmap(b []byte) error {
	if b != nil {
		return syscall.Munmap(b)
	}
	return nil
}
