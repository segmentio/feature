package feature

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Iter is an interface implemented by the iterator types exposed by this
// package.
type Iter interface {
	Close() error
	Next() bool
	Name() string
}

// Scan is a helper function used to iterate over each name exposed by an
// iterator.
func Scan(it Iter, do func(string) error) error {
	defer it.Close()

	for it.Next() {
		if err := do(it.Name()); err != nil {
			return err
		}
	}

	return it.Close()
}

// MountPoint represents the mount point of a feature file system.
//
// The type is a simple string alias, its value is the path to the root
// directory of the file system.
type MountPoint string

func Mount(path string) (MountPoint, error) {
	p, err := filepath.Abs(path)
	return MountPoint(p), err
}

func (path MountPoint) CreateTier(group, name string) (*Tier, error) {
	if err := mkdir(path.groupPath(group)); err != nil {
		return nil, fmt.Errorf("creating tier group %q: %w", group, err)
	}
	if err := mkdir(path.tierPath(group, name)); err != nil {
		return nil, fmt.Errorf("creating tier %q of group %q: %w", name, group, err)
	}
	return &Tier{path: path, group: group, name: name}, nil
}

func (path MountPoint) OpenTier(group, name string) (*Tier, error) {
	_, err := os.Stat(path.tierPath(group, name))
	if err != nil {
		return nil, err
	}
	return &Tier{path: path, group: group, name: name}, nil
}

func (path MountPoint) DeleteTier(group, name string) error {
	return rmdir(path.tierPath(group, name))
}

func (path MountPoint) DeleteGroup(group string) error {
	return rmdir(path.groupPath(group))
}

func (path MountPoint) Tiers(group string) *TierIter {
	return &TierIter{readdir(path.groupPath(group))}
}

func (path MountPoint) Groups() *GroupIter {
	return &GroupIter{readdir(string(path))}
}

func (path MountPoint) groupPath(group string) string {
	return filepath.Join(string(path), group)
}

func (path MountPoint) tierPath(group, name string) string {
	return filepath.Join(string(path), group, name)
}

type dir struct {
	file  *os.File
	path  string
	names []string
	index int
	err   error
}

func (d *dir) opened() bool {
	return d.file != nil
}

func (d *dir) close() error {
	if d.file != nil {
		d.file.Close()
		d.file = nil
	}
	d.file = nil
	d.names = nil
	d.index = 0
	return d.err
}

func (d *dir) next() bool {
	if d.file == nil {
		return false
	}

	if d.index++; d.index < len(d.names) {
		return true
	}

	names, err := d.file.Readdirnames(100)
	switch err {
	case nil:
		d.names, d.index = names, 0
		return true
	case io.EOF:
		d.names, d.index = nil, 0
		return false
	default:
		d.err = err
		d.close()
		return false
	}
}

func (d *dir) name() string {
	if d.index >= 0 && d.index < len(d.names) {
		return d.names[d.index]
	}
	return ""
}

func (d *dir) read() dir {
	return readdir(filepath.Join(d.path, d.name()))
}

func mkdir(path string) error {
	err := os.Mkdir(path, 0755)
	if err != nil && os.IsExist(err) {
		err = nil
	}
	return err
}

func readdir(path string) dir {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
	}
	return dir{path: path, file: f, err: err}
}

func rmdir(path string) error {
	err := os.RemoveAll(path)
	if os.IsNotExist(err) {
		err = nil
	}
	return err
}

func unlink(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		err = nil
	}
	return err
}

func writeFile(path string, write func(*os.File) error) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return write(f)
}
