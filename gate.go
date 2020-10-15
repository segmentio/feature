package feature

import (
	"bytes"
	"hash"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"unicode"
)

type FamilyIter struct{ dir }

func (it *FamilyIter) Close() error { return it.close() }

func (it *FamilyIter) Next() bool { return it.next() }

func (it *FamilyIter) Name() string { return it.name() }

func (it *FamilyIter) Gates() *GateIter { return &GateIter{it.read()} }

type GateIter struct{ dir }

func (it *GateIter) Close() error { return it.close() }

func (it *GateIter) Next() bool { return it.next() }

func (it *GateIter) Name() string { return it.name() }

func (it *GateIter) Created() *GateCreatedIter { return &GateCreatedIter{it.read()} }

type GateCreatedIter struct{ dir }

func (it *GateCreatedIter) Close() error { return it.close() }

func (it *GateCreatedIter) Next() bool { return it.next() }

func (it *GateCreatedIter) Name() string { return it.name() }

type GateEnabledIter struct {
	path       MountPoint
	families   dir
	gates      dir
	collection string
	id         string
	salt       uint32
	err        error
	hash       *bufferedHash64
}

func (it *GateEnabledIter) Close() error {
	if it.hash != nil {
		releaseBufferedHash64(it.hash)
		it.hash = nil
	}
	err1 := it.families.close()
	err2 := it.gates.close()
	if err2 != nil {
		return err2
	}
	if err1 != nil {
		return err1
	}
	return it.err
}

func (it *GateEnabledIter) Next() bool {
	if it.hash == nil {
		it.hash = acquireBufferedHash64()
	}

	for {
		if it.gates.opened() {
			for it.gates.next() {
				g, err := readGate(filepath.Join(it.gates.path, it.gates.name(), it.collection))
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					it.err = err
					it.Close()
					return false
				}

				if openGate(it.id, g.salt, g.volume, it.hash) {
					return true
				}
			}

			if it.gates.close() != nil {
				return false
			}
		}

		if !it.families.next() {
			return false
		}

		it.gates = it.families.read()
	}
}

func (it *GateEnabledIter) Family() string {
	return it.families.name()
}

func (it *GateEnabledIter) Gate() string {
	return it.gates.name()
}

func (it *GateEnabledIter) Name() string {
	return it.Family() + "/" + it.Gate()
}

// openGate is inherited from github.com/segmentio/flagon; we had to port the
// algorithm to ensure compatibility between the packages.
func openGate(id, salt string, volume float64, h *bufferedHash64) bool {
	if volume <= 0 {
		return false
	}

	if volume >= 1 {
		return true
	}

	h.buffer.WriteString(id)
	h.buffer.WriteString(salt)
	h.buffer.WriteTo(h.hash)

	return (float64(h.hash.Sum64()%100) + 1) <= (100 * volume)
}

type gate struct {
	salt   string
	volume float64
}

func readGate(path string) (gate, error) {
	var g gate

	f, err := os.Open(path)
	if err != nil {
		return g, err
	}
	defer f.Close()

	b, err := mmap(f)
	if err != nil {
		return g, err
	}
	defer munmap(b)

	forEachLine(b, func(i, n int) {
		if err != nil {
			return
		}
		k, v := splitKeyValue(bytes.TrimSpace(b[i : i+n]))
		switch string(k) {
		case "salt":
			g.salt = string(v)
		case "volume":
			g.volume, err = strconv.ParseFloat(string(v), 64)
		}
	})

	if err != nil {
		err = &os.PathError{Op: "read", Path: path, Err: err}
	}

	return g, err
}

func writeGate(path string, gate gate) error {
	b := make([]byte, 0, 128)

	if gate.salt != "" {
		b = append(b, "salt\t"...)
		b = append(b, gate.salt...)
		b = append(b, '\n')
	}

	if gate.volume != 0 {
		b = append(b, "volume\t"...)
		b = strconv.AppendFloat(b, gate.volume, 'g', -1, 64)
		b = append(b, '\n')
	}

	return writeFile(path, func(f *os.File) error {
		_, err := f.Write(b)
		return err
	})
}

func splitKeyValue(line []byte) ([]byte, []byte) {
	i := bytes.IndexFunc(line, unicode.IsSpace)
	if i < 0 {
		return bytes.TrimSpace(line), nil
	}
	return bytes.TrimSpace(line[:i]), bytes.TrimSpace(line[i:])
}

var hashes sync.Pool // *bufferedHash64

type bufferedHash64 struct {
	buffer bytes.Buffer
	hash   hash.Hash64
}

func acquireBufferedHash64() *bufferedHash64 {
	h, _ := hashes.Get().(*bufferedHash64)
	if h == nil {
		h = &bufferedHash64{hash: fnv.New64a()}
		h.buffer.Grow(128)
	} else {
		h.buffer.Reset()
		h.hash.Reset()
	}
	return h
}

func releaseBufferedHash64(h *bufferedHash64) {
	hashes.Put(h)
}
