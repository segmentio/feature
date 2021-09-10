package feature

import (
	"bytes"
	"container/list"
	"hash/maphash"
	"os"
	"path/filepath"
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
	cache lruCache
	mutex sync.RWMutex
	tiers []cachedTier
}

func (c *Cache) swap(x *Cache) *Cache {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.tiers, x.tiers = x.tiers, c.tiers
	c.cache.clear()
	return x
}

// Close releases resources held by the cache.
func (c *Cache) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for i := range c.tiers {
		for _, c := range c.tiers[i].collections {
			c.unmap()
		}
	}

	c.tiers = nil
	c.cache.clear()
	return nil
}

// GateOpen returns true if a gate is opened for a given id.
//
// The method does not retain any of the strings passed as arguments.
func (c *Cache) GateOpen(family, gate, collection, id string) bool {
	g := c.LookupGates(family, collection, id)
	i := sort.Search(len(g), func(i int) bool {
		return g[i] >= gate
	})
	return i < len(g) && g[i] == gate
}

// LookupGates returns the list of open gates in a family for a given id.
//
// The method does not retain any of the strings passed as arguments.
func (c *Cache) LookupGates(family, collection, id string) []string {
	key := lruCacheKey{
		family:     family,
		collection: collection,
		id:         id,
	}

	if v := c.cache.lookup(key); v != nil && v.key == key {
		return v.gates
	}

	buf := family + collection + id
	key = lruCacheKey{
		family:     buf[:len(family)],
		collection: buf[len(family) : len(family)+len(collection)],
		id:         buf[len(family)+len(collection):],
	}

	disabled := make(map[string]struct{})
	gates := make([]string, 0, 8)
	defer func() {
		c.cache.insert(key, gates, 4096)
	}()

	h := acquireBufferedHash64()
	defer releaseBufferedHash64(h)

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for i := range c.tiers {
		t := &c.tiers[i]
		c := t.collections[collection]
		exists := c != nil && c.contains(id)

		for _, g := range t.gates[family] {
			if g.collection == collection {
				if exists {
					if openGate(id, g.salt, g.volume, h) {
						gates = append(gates, g.name)
					} else {
						disabled[g.name] = struct{}{}
					}
				} else if g.open {
					gates = append(gates, g.name)
				}
			}
		}
	}

	if len(gates) == 0 {
		gates = nil
	} else {
		sort.Strings(gates)
		gates = deduplicate(gates)
		gates = strip(gates, disabled)
		// Safe guard in case the program appends to the slice, it will force
		// the reallocation and copy.
		gates = gates[:len(gates):len(gates)]
	}

	return gates
}

func deduplicate(s []string) []string {
	n := 0

	for i := 1; i < len(s); i++ {
		if s[i] != s[n] {
			n++
			s[n] = s[i]
		}
	}

	for i := n + 1; i < len(s); i++ {
		s[i] = ""
	}

	return s[:n+1]
}

func strip(s []string, disabled map[string]struct{}) []string {
	n := 0

	for _, x := range s {
		if _, skip := disabled[x]; !skip {
			s[n] = x
			n++
		}
	}

	for i := n; i < len(s); i++ {
		s[i] = ""
	}

	return s[:n]
}

type cachedTier struct {
	group       string
	name        string
	collections map[string]*collection
	gates       map[string][]cachedGate
}

type cachedGate struct {
	name       string
	collection string
	salt       string
	volume     float64
	open       bool
}

// The Load method loads the features at the mount point it is called on,
// returning a Cache object exposing the state.
//
// The returned cache holds operating system resources and therefore must be
// closed when the program does not need it anymore.
func (path MountPoint) Load() (*Cache, error) {
	// Resolves symlinks first so we know that the underlying directory
	// structure will not change across reads from the file system when
	// loading the cache.
	p, err := filepath.EvalSymlinks(string(path))
	if err != nil {
		return nil, err
	}
	path = MountPoint(p)
	// To minimize the memory footprint of the cache, strings are deduplicated
	// using this map, so we only retain only one copy of each string value.
	strings := stringCache{}

	tiers := make([]cachedTier, 0)

	if err := Scan(path.Groups(), func(group string) error {
		return Scan(path.Tiers(group), func(tier string) error {
			t, err := path.OpenTier(group, tier)
			if err != nil {
				return err
			}
			defer t.Close()

			c := cachedTier{
				group:       strings.load(group),
				name:        strings.load(tier),
				collections: make(map[string]*collection),
				gates:       make(map[string][]cachedGate),
			}

			if err := Scan(t.Families(), func(family string) error {
				return Scan(t.Gates(family), func(gate string) error {
					f := strings.load(family)
					d := readdir(t.gatePath(family, gate))
					defer d.close()

					for d.next() {
						open, salt, volume, err := t.ReadGate(family, gate, d.name())
						if err != nil {
							return err
						}
						c.gates[f] = append(c.gates[f], cachedGate{
							name:       strings.load(gate),
							collection: strings.load(d.name()),
							salt:       salt,
							volume:     volume,
							open:       open,
						})
					}

					return nil
				})
			}); err != nil {
				return err
			}

			if err := Scan(t.Collections(), func(collection string) error {
				col, err := mmapCollection(t.collectionPath(collection))
				if err != nil {
					return err
				}
				c.collections[strings.load(collection)] = col
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

type collection struct {
	memory []byte
	index  []slice
}

func (col *collection) at(i int) []byte {
	slice := col.index[i]
	return col.memory[slice.offset : slice.offset+slice.length]
}

func (col *collection) contains(id string) bool {
	i := sort.Search(len(col.index), func(i int) bool {
		return string(col.at(i)) >= id
	})
	return i < len(col.index) && string(col.at(i)) == id
}

func (col *collection) unmap() {
	munmap(col.memory)
	col.memory, col.index = nil, nil
}

func (col *collection) Len() int           { return len(col.index) }
func (col *collection) Less(i, j int) bool { return string(col.at(i)) < string(col.at(j)) }
func (col *collection) Swap(i, j int)      { col.index[i], col.index[j] = col.index[j], col.index[i] }

func mmapCollection(path string) (*collection, error) {
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

	col := &collection{memory: m, index: index}
	if !sort.IsSorted(col) {
		sort.Sort(col)
	}
	return col, nil
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

type stringCache map[string]string

func (c stringCache) load(s string) string {
	v, ok := c[s]
	if ok {
		return v
	}
	c[s] = s
	return s
}

var lruCacheSeed = maphash.MakeSeed()

type lruCacheKey struct {
	family     string
	collection string
	id         string
}

func (k *lruCacheKey) hash(h *maphash.Hash) uint64 {
	h.WriteString(k.family)
	h.WriteString(k.collection)
	h.WriteString(k.id)
	return h.Sum64()
}

type lruCacheValue struct {
	key   lruCacheKey
	gates []string
}

type lruCache struct {
	mutex sync.Mutex
	queue list.List
	cache map[uint64]*list.Element
}

func (c *lruCache) clear() {
	c.mutex.Lock()
	c.queue = list.List{}
	for key := range c.cache {
		delete(c.cache, key)
	}
	c.mutex.Unlock()
}

func (c *lruCache) insert(key lruCacheKey, gates []string, limit int) {
	m := maphash.Hash{}
	m.SetSeed(lruCacheSeed)
	h := key.hash(&m)

	v := &lruCacheValue{
		key:   key,
		gates: gates,
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.cache == nil {
		c.cache = make(map[uint64]*list.Element)
	}

	e := c.queue.PushBack(v)
	c.cache[h] = e

	for limit > 0 && len(c.cache) > limit {
		e := c.queue.Back()
		v := e.Value.(*lruCacheValue)
		c.queue.Remove(e)
		m.Reset()
		delete(c.cache, v.key.hash(&m))
	}
}

func (c *lruCache) lookup(key lruCacheKey) *lruCacheValue {
	m := maphash.Hash{}
	m.SetSeed(lruCacheSeed)
	h := key.hash(&m)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	e := c.cache[h]
	if e != nil {
		c.queue.MoveToFront(e)
		return e.Value.(*lruCacheValue)
	}

	return nil
}
