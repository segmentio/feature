package feature_test

import (
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/segmentio/feature"
)

func TestGate(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T, feature.MountPoint, *feature.Tier)
	}{
		{
			scenario: "fully enabled gates are exposed when listing gates for a tier",
			function: testTierGateEnabled,
		},

		{
			scenario: "fully disabled gates are not exposed when listing gates for a tier",
			function: testTierGateDisabled,
		},

		{
			scenario: "disabling a gate family in a tier causes all its gates to not be exposed anymore",
			function: testTierGateDisableFamily,
		},

		{
			scenario: "disabling a gate family in a tier causes all its gates to not be exposed anymore",
			function: testTierGateDisableFamily,
		},

		{
			scenario: "disabling all gates in a tier causes all its gates to not be exposed anymore",
			function: testTierGateDisableAll,
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			tmp, err := ioutil.TempDir("", "feature")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmp)
			path := feature.MountPoint(tmp)

			tier, err := path.CreateTier("standard", "1")
			if err != nil {
				t.Fatal(err)
			}
			defer tier.Close()

			test.function(t, path, tier)
		})
	}
}

func testTierGateEnabled(t *testing.T, path feature.MountPoint, tier *feature.Tier) {
	g1 := createGate(t, path, "family-A", "gate-1")
	g2 := createGate(t, path, "family-A", "gate-2")
	defer g1.Close()
	defer g2.Close()

	col := createCollection(t, tier, "collection")
	defer col.Close()

	populateCollection(t, col, []string{
		"id-1",
		"id-2",
		"id-3",
	})

	enableGate(t, tier, "family-A", "gate-1", 1.0)
	enableGate(t, tier, "family-A", "gate-2", 1.0)
	gates := map[string][]string{
		"family-A": {"gate-1", "gate-2"},
	}

	expectGatesEnabled(t, tier, "collection", "id-1", gates)
	expectGatesEnabled(t, tier, "collection", "id-2", gates)
	expectGatesEnabled(t, tier, "collection", "id-3", gates)
}

func testTierGateDisabled(t *testing.T, path feature.MountPoint, tier *feature.Tier) {
	g1 := createGate(t, path, "family-A", "gate-1")
	g2 := createGate(t, path, "family-A", "gate-2")
	defer g1.Close()
	defer g2.Close()

	col := createCollection(t, tier, "collection")
	defer col.Close()

	populateCollection(t, col, []string{
		"id-1",
		"id-2",
		"id-3",
	})

	enableGate(t, tier, "family-A", "gate-1", 1.0)
	enableGate(t, tier, "family-A", "gate-2", 0.0)
	gates := map[string][]string{
		"family-A": {"gate-1"},
	}

	expectGatesEnabled(t, tier, "collection", "id-1", gates)
	expectGatesEnabled(t, tier, "collection", "id-2", gates)
	expectGatesEnabled(t, tier, "collection", "id-3", gates)
}

func testTierGateDisableFamily(t *testing.T, path feature.MountPoint, tier *feature.Tier) {
	g1 := createGate(t, path, "family-A", "gate-1")
	g2 := createGate(t, path, "family-A", "gate-1")
	g3 := createGate(t, path, "family-B", "gate-3")
	defer g1.Close()
	defer g2.Close()
	defer g3.Close()

	col := createCollection(t, tier, "collection")
	defer col.Close()

	populateCollection(t, col, []string{
		"id-1",
		"id-2",
		"id-3",
	})

	enableGate(t, tier, "family-A", "gate-1", 1.0)
	enableGate(t, tier, "family-A", "gate-2", 1.0)
	enableGate(t, tier, "family-B", "gate-3", 1.0)
	gates := map[string][]string{
		"family-B": {"gate-3"},
	}

	if err := tier.DisableFamily("family-A"); err != nil {
		t.Fatal(err)
	}

	expectGatesEnabled(t, tier, "collection", "id-1", gates)
	expectGatesEnabled(t, tier, "collection", "id-2", gates)
	expectGatesEnabled(t, tier, "collection", "id-3", gates)
}

func testTierGateDisableAll(t *testing.T, path feature.MountPoint, tier *feature.Tier) {
	g1 := createGate(t, path, "family-A", "gate-1")
	g2 := createGate(t, path, "family-A", "gate-1")
	g3 := createGate(t, path, "family-B", "gate-3")
	defer g1.Close()
	defer g2.Close()
	defer g3.Close()

	col := createCollection(t, tier, "collection")
	defer col.Close()

	populateCollection(t, col, []string{
		"id-1",
		"id-2",
		"id-3",
	})

	enableGate(t, tier, "family-A", "gate-1", 1.0)
	enableGate(t, tier, "family-A", "gate-2", 1.0)
	enableGate(t, tier, "family-B", "gate-3", 1.0)
	gates := map[string][]string{}

	if err := tier.DisableAll(); err != nil {
		t.Fatal(err)
	}

	expectGatesEnabled(t, tier, "collection", "id-1", gates)
	expectGatesEnabled(t, tier, "collection", "id-2", gates)
	expectGatesEnabled(t, tier, "collection", "id-3", gates)
}

func enableGate(t testing.TB, tier *feature.Tier, family, gate string, volume float64) {
	t.Helper()

	if err := tier.EnableGate(family, gate, volume); err != nil {
		t.Error("unexpected error enabling gate:", err)
	}
}

func expectGatesEnabled(t testing.TB, tier *feature.Tier, collection, id string, gates map[string][]string) {
	t.Helper()

	it := tier.GatesEnabled(collection, id)
	defer it.Close()

	found := make(map[string][]string, len(gates))
	for it.Next() {
		found[it.Family()] = append(found[it.Family()], it.Gate())
	}

	for _, gates := range found {
		sort.Strings(gates)
	}

	if !reflect.DeepEqual(gates, found) {
		t.Error("gates mismatch")
		t.Logf("want: %+v", gates)
		t.Logf("got:  %+v", found)
	}
}
