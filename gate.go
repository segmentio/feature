package feature

import (
	"bytes"
	"fmt"
	"hash"
	"hash/fnv"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
				s, err := readGateSalt(filepath.Join(it.gates.path, it.gates.name(), it.collection, "salt"))
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					it.err = err
					it.Close()
					return false
				}

				v, err := readGateVolume(filepath.Join(it.gates.path, it.gates.name(), it.collection, "volume"))
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					it.err = err
					it.Close()
					return false
				}

				if openGate(it.id, s, v, it.hash) {
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

func readGateFile(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}

func readGateSalt(path string) (string, error) {
	b, err := readGateFile(path)
	return strings.TrimSpace(string(b)), err
}

func readGateVolume(path string) (float64, error) {
	b, err := readGateFile(path)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if err != nil {
		return 0, err
	}
	if v < 0 || v > 1 {
		return 0, fmt.Errorf("invalid tier gate value out of range: %g", v)
	}
	return v, nil
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
