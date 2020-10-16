package feature_test

import (
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/segmentio/feature"
)

func TestCache(t *testing.T) {
	tmp, err := ioutil.TempDir("", "feature")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	path := feature.MountPoint(tmp)

	tier1 := createTier(t, path, "standard", "1")
	tier2 := createTier(t, path, "standard", "2")
	tier3 := createTier(t, path, "standard", "3")

	defer tier1.Close()
	defer tier2.Close()
	defer tier3.Close()

	col1 := createCollection(t, tier1, "workspaces")
	col2 := createCollection(t, tier2, "workspaces")
	col3 := createCollection(t, tier3, "workspaces")

	defer col1.Close()
	defer col2.Close()
	defer col3.Close()

	populateCollection(t, col1, []string{"id-1"})
	populateCollection(t, col2, []string{"id-2", "id-3"})
	populateCollection(t, col3, []string{"id-4", "id-5", "id-6"})

	createGate(t, tier1, "family-A", "gate-1", "workspaces", 1234)
	createGate(t, tier1, "family-A", "gate-2", "workspaces", 2345)
	createGate(t, tier2, "family-B", "gate-3", "workspaces", 3456)

	enableGate(t, tier1, "family-A", "gate-1", "workspaces", 1.0, false)
	enableGate(t, tier1, "family-A", "gate-2", "workspaces", 1.0, false)
	enableGate(t, tier2, "family-B", "gate-3", "workspaces", 1.0, false)

	cache, err := path.Load()
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()

	expectGateOpened(t, cache, "family-A", "gate-1", "workspaces", "id-1")
	expectGateClosed(t, cache, "family-A", "gate-1", "workspaces", "id-2")
	expectGateLookup(t, cache, "family-A", "workspaces", "id-1", []string{"gate-1", "gate-2"})
}

func expectGateOpened(t testing.TB, cache *feature.Cache, family, gate, collection, id string) {
	t.Helper()
	expectGateIsEnabled(t, cache, family, gate, collection, id, true)
}

func expectGateClosed(t testing.TB, cache *feature.Cache, family, gate, collection, id string) {
	t.Helper()
	expectGateIsEnabled(t, cache, family, gate, collection, id, false)
}

func expectGateIsEnabled(t testing.TB, cache *feature.Cache, family, gate, collection, id string, open bool) {
	t.Helper()

	if cache.GateOpen(family, gate, collection, id) != open {
		t.Error("gate state mismatch")
		t.Logf("want: %t", open)
		t.Logf("got:  %t", !open)
	}
}

func expectGateLookup(t testing.TB, cache *feature.Cache, family, collection, id string, gates []string) {
	t.Helper()

	if found := cache.LookupGates(family, collection, id); !reflect.DeepEqual(found, gates) {
		t.Error("gates mismatch")
		t.Logf("want: %q", gates)
		t.Logf("got:  %q", found)
	}
}

func BenchmarkCache(b *testing.B) {
	tmp, err := ioutil.TempDir("", "feature")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	path := feature.MountPoint(tmp)

	tier1 := createTier(b, path, "standard", "1")
	tier2 := createTier(b, path, "standard", "2")

	defer tier1.Close()
	defer tier2.Close()

	col1 := createCollection(b, tier1, "workspaces")
	col2 := createCollection(b, tier2, "workspaces")

	defer col1.Close()
	defer col2.Close()

	createGate(b, tier1, "family-A", "gate-1", "workspaces", 1234)
	createGate(b, tier1, "family-A", "gate-2", "workspaces", 2345)

	createGate(b, tier2, "family-A", "gate-1", "workspaces", 1234)
	createGate(b, tier2, "family-A", "gate-2", "workspaces", 2345)

	enableGate(b, tier1, "family-A", "gate-1", "workspaces", 1.0, false)
	enableGate(b, tier1, "family-A", "gate-2", "workspaces", 1.0, false)

	enableGate(b, tier2, "family-A", "gate-1", "workspaces", 0.0, false)
	enableGate(b, tier2, "family-A", "gate-2", "workspaces", 0.0, false)

	const N = 10e3
	prng := rand.New(rand.NewSource(0))
	ids := make([]string, N)
	for i := range ids {
		ids[i] = strconv.FormatInt(prng.Int63(), 16)
	}

	populateCollection(b, col1, ids[:N/2])
	populateCollection(b, col2, ids[N/2:])

	cache, err := path.Load()
	if err != nil {
		b.Fatal(err)
	}
	defer cache.Close()

	b.Run("enabled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if !cache.GateOpen("family-A", "gate-2", "workspaces", ids[i%(N/2)]) {
				b.Fatal("gate not enabled")
			}
		}
	})

	b.Run("disabled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if cache.GateOpen("family-A", "gate-2", "workspaces", ids[(N/2)+i%(N/2)]) {
				b.Fatal("gate not disabled")
			}
		}
	})
}
