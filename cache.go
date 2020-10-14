package feature

import (
	"bytes"
	"os"
	"sort"
	"sync"
)

// Cache is an in-memory view of feature mount point on a file system.
//
// The cache is optimized for fast lookups of gates, and fast test of gate
// open states for an id. The cache is also immutable, and therefore safe to
// use concurrently from multiple goroutines.
//
// The cache is designed to minimize the memory footprint. The underlying files
// containing the id collections are memory mapped so multiple programs are able
// to share the memory pages.
type Cache struct {
	mutex sync.RWMutex
	tiers []cachedTier
}

// Close releases resources held by the cache.
func (c *Cache) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for i := range c.tiers {
		for _, ids := range c.tiers[i].ids {
			ids.unmap()
		}
	}

	c.tiers = nil
	return nil
}

// GateOpened returns true if a gate is opened for a given id.
func (c *Cache) GateOpen(family, gate, collection, id string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for i := range c.tiers {
		t := &c.tiers[i]

		if t.ids[collection].contains(id) {
			f := t.gates[family]
			i := sort.Search(len(f), func(i int) bool {
				return f[i].name >= gate
			})

			for _, g := range f[i:] {
				if g.name != gate {
					break
				}
				if g.collection == collection && g.open(id) {
					return true
				}
			}

			return false
		}
	}

	return false
}

// AppendGates appends the list of open gates in a family for a given id.
func (c *Cache) AppendGates(gates []string, family, collection, id string) []string {
	h := acquireBufferedHash64()
	defer releaseBufferedHash64(h)

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for i := range c.tiers {
		t := &c.tiers[i]

		if t.ids[collection].contains(id) {
			for _, g := range t.gates[family] {
				if g.collection == collection && g.openWith(id, h) {
					gates = append(gates, g.name)
				}
			}
			break
		}
	}

	return gates
}

// LookupGates returns the list of open gates in a family for a given id.
func (c *Cache) LookupGates(family, collection, id string) []string {
	return c.AppendGates(make([]string, 0, 8), family, collection, id)
}

type cachedTier struct {
	group string
	name  string
	ids   map[string]*idset
	gates map[string][]cachedGate
}

type cachedGate struct {
	name       string
	collection string
	salt       string
	volume     float64
}

func (g *cachedGate) open(id string) bool {
	h := acquireBufferedHash64()
	defer releaseBufferedHash64(h)
	return g.openWith(id, h)
}

func (g *cachedGate) openWith(id string, h *bufferedHash64) bool {
	return openGate(id, g.salt, g.volume, h)
}

// The Laod method loads the features at the mount point it is called on,
// returning a Cache object exposing the state.
//
// The returned cache holds operating system resources and therefore must be
// closed when the program does not need it anymore.
func (path MountPoint) Load() (*Cache, error) {
	// To minimize the memory footprint of the cache, strings are deduplicated
	// using this map, so we only retain only one copy of each string value.
	strings := stringset{}

	tiers := make([]cachedTier, 0)

	if err := Scan(path.Groups(), func(group string) error {
		return Scan(path.Tiers(group), func(tier string) error {
			t, err := path.OpenTier(group, tier)
			if err != nil {
				return err
			}
			defer t.Close()

			c := cachedTier{
				group: strings.load(group),
				name:  strings.load(tier),
				ids:   make(map[string]*idset),
				gates: make(map[string][]cachedGate),
			}

			if err := Scan(t.Families(), func(family string) error {
				return Scan(t.Gates(family), func(gate string) error {
					f := strings.load(family)
					d := readdir(t.gatePath(family, gate))
					defer d.close()

					for d.next() {
						salt, volume, err := t.ReadGate(family, gate, d.name())
						if err != nil {
							return err
						}
						c.gates[f] = append(c.gates[f], cachedGate{
							name:       strings.load(gate),
							collection: strings.load(d.name()),
							salt:       salt,
							volume:     volume,
						})
					}

					return nil
				})
			}); err != nil {
				return err
			}

			if err := Scan(t.Collections(), func(collection string) error {
				ids, err := mmapIDs(t.collectionPath(collection))
				if err != nil {
					return err
				}
				c.ids[strings.load(collection)] = ids
				return nil
			}); err != nil {
				return err
			}

			tiers = append(tiers, c)
			return nil
		})
	}); err != nil {
		return nil, err
	}

	for _, tier := range tiers {
		for _, gates := range tier.gates {
			sort.Slice(gates, func(i, j int) bool {
				return gates[i].name < gates[j].name
			})
		}
	}

	return &Cache{tiers: tiers}, nil
}

type slice struct {
	offset uint32
	length uint32
}

type idset struct {
	memory []byte
	index  []slice
}

func (ids *idset) at(i int) []byte {
	slice := ids.index[i]
	return ids.memory[slice.offset : slice.offset+slice.length]
}

func (ids *idset) contains(id string) bool {
	i := sort.Search(len(ids.index), func(i int) bool {
		return string(ids.at(i)) >= id
	})
	return i < len(ids.index) && string(ids.at(i)) == id
}

func (ids *idset) unmap() {
	munmap(ids.memory)
	ids.memory, ids.index = nil, nil
}

func (ids *idset) Len() int           { return len(ids.index) }
func (ids *idset) Less(i, j int) bool { return string(ids.at(i)) < string(ids.at(j)) }
func (ids *idset) Swap(i, j int)      { ids.index[i], ids.index[j] = ids.index[j], ids.index[i] }

func mmapIDs(path string) (*idset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m, err := mmap(f)
	if err != nil {
		return nil, err
	}

	count := 0
	forEachLine(m, func(int, int) { count++ })

	index := make([]slice, 0, count)
	forEachLine(m, func(off, len int) {
		index = append(index, slice{
			offset: uint32(off),
			length: uint32(len),
		})
	})

	ids := &idset{memory: m, index: index}
	if !sort.IsSorted(ids) {
		sort.Sort(ids)
	}
	return ids, nil
}

func forEachLine(b []byte, do func(off, len int)) {
	for i := 0; i < len(b); {
		n := bytes.IndexByte(b[i:], '\n')
		if n < 0 {
			n = len(b) - i
		}
		do(i, n)
		i += n + 1
	}
}

type stringset map[string]string

func (c stringset) load(s string) string {
	v, ok := c[s]
	if ok {
		return v
	}
	c[s] = s
	return s
}
