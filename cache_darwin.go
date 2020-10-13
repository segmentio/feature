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
	return syscall.Mmap(int(f.Fd()), 0, int(s.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
}

func munmap(b []byte) error {
	return syscall.Munmap(b)
}
