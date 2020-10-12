package feature

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func (path MountPoint) CreateGate(family, name string, salt uint32) (*Gate, error) {
	if err := mkdir(path.to("gates")); err != nil {
		return nil, err
	}

	if err := mkdir(path.familyPath(family)); err != nil {
		return nil, fmt.Errorf("creating gate family %q: %w", family, err)
	}

	if err := writeFile(path.gatePath(family, name), func(f *os.File) error {
		_, err := fmt.Fprintf(f, "%d\n", salt)
		return err
	}); err != nil {
		return nil, fmt.Errorf("creating gate %q of family %q: %w", name, family, err)
	}

	g := &Gate{
		path:   path,
		family: family,
		name:   name,
		salt:   strconv.FormatUint(uint64(salt), 10),
	}

	return g, nil
}

func (path MountPoint) OpenGate(family, name string) (*Gate, error) {
	b, err := ioutil.ReadFile(path.gatePath(family, name))
	if err != nil {
		if os.IsNotExist(err) {
			// Make a special case so the caller can use os.IsNotExist to test
			// the error.
			return nil, err
		}
		return nil, fmt.Errorf("opening gate %q of family %q: %w", name, family, err)
	}

	g := &Gate{
		path:   path,
		family: family,
		name:   name,
		salt:   strings.TrimSpace(string(b)),
	}

	return g, nil
}

func (path MountPoint) CreateTier(group, name string) (*Tier, error) {
	if err := mkdir(path.to("tiers")); err != nil {
		return nil, err
	}

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

func (path MountPoint) DeleteGate(family, name string) error {
	return rmdir(path.gatePath(family, name))
}

func (path MountPoint) DeleteFamily(family string) error {
	return rmdir(path.familyPath(family))
}

func (path MountPoint) DeleteTier(group, name string) error {
	return rmdir(path.tierPath(group, name))
}

func (path MountPoint) DeleteGroup(group string) error {
	return rmdir(path.groupPath(group))
}

func (path MountPoint) Gates(family string) *GateIter {
	return &GateIter{readdir(path.familyPath(family))}
}

func (path MountPoint) Families() *FamilyIter {
	return &FamilyIter{readdir(path.to("gates"))}
}

func (path MountPoint) Tiers(group string) *TierIter {
	return &TierIter{readdir(path.groupPath(group))}
}

func (path MountPoint) Groups() *GroupIter {
	return &GroupIter{readdir(path.to("tiers"))}
}

func (path MountPoint) familyPath(family string) string {
	return filepath.Join(string(path), "gates", family)
}

func (path MountPoint) gatePath(family, name string) string {
	return filepath.Join(string(path), "gates", family, name)
}

func (path MountPoint) groupPath(group string) string {
	return filepath.Join(string(path), "tiers", group)
}

func (path MountPoint) tierPath(group, name string) string {
	return filepath.Join(string(path), "tiers", group, name)
}

func (path MountPoint) to(name string) string {
	return filepath.Join(string(path), name)
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

	names, err := d.file.Readdirnames(32)
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
