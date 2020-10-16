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
			scenario: "enabled gates are exposed when listing gates for a tier",
			function: testTierGateEnabled,
		},

		{
			scenario: "disabled gates are not exposed when listing gates for a tier",
			function: testTierGateDisabled,
		},

		{
			scenario: "deleted gates are not exposed when listing gates for a tier",
			function: testTierGateDelete,
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
	col := createCollection(t, tier, "collection")
	defer col.Close()

	populateCollection(t, col, []string{
		"id-1",
		"id-2",
		"id-3",
	})

	createGate(t, tier, "family-A", "gate-1", "collection", 1234)
	createGate(t, tier, "family-A", "gate-2", "collection", 2345)

	enableGate(t, tier, "family-A", "gate-1", "collection", 1.0, false)
	enableGate(t, tier, "family-A", "gate-2", "collection", 1.0, false)
	gates := map[string][]string{
		"family-A": {"gate-1", "gate-2"},
	}

	expectGatesEnabled(t, tier, "collection", "id-1", gates)
	expectGatesEnabled(t, tier, "collection", "id-2", gates)
	expectGatesEnabled(t, tier, "collection", "id-3", gates)
}

func testTierGateDisabled(t *testing.T, path feature.MountPoint, tier *feature.Tier) {
	col := createCollection(t, tier, "collection")
	defer col.Close()

	populateCollection(t, col, []string{
		"id-1",
		"id-2",
		"id-3",
	})

	createGate(t, tier, "family-A", "gate-1", "collection", 1234)
	createGate(t, tier, "family-A", "gate-2", "collection", 2345)

	enableGate(t, tier, "family-A", "gate-1", "collection", 0.0, false)
	enableGate(t, tier, "family-A", "gate-2", "collection", 0.0, false)
	gates := map[string][]string{}

	expectGatesEnabled(t, tier, "collection", "id-1", gates)
	expectGatesEnabled(t, tier, "collection", "id-2", gates)
	expectGatesEnabled(t, tier, "collection", "id-3", gates)
}

func testTierGateDelete(t *testing.T, path feature.MountPoint, tier *feature.Tier) {
	col := createCollection(t, tier, "collection")
	defer col.Close()

	populateCollection(t, col, []string{
		"id-1",
		"id-2",
		"id-3",
	})

	createGate(t, tier, "family-A", "gate-1", "collection", 1234)
	createGate(t, tier, "family-A", "gate-2", "collection", 2345)

	enableGate(t, tier, "family-A", "gate-1", "collection", 1.0, false)
	enableGate(t, tier, "family-A", "gate-2", "collection", 0.0, false)
	gates := map[string][]string{
		"family-A": {"gate-1"},
	}

	expectGatesEnabled(t, tier, "collection", "id-1", gates)
	expectGatesEnabled(t, tier, "collection", "id-2", gates)
	expectGatesEnabled(t, tier, "collection", "id-3", gates)
}

func createGate(t testing.TB, tier *feature.Tier, family, gate, collection string, salt uint32) {
	t.Helper()

	if err := tier.CreateGate(family, gate, collection, salt); err != nil {
		t.Error("unexpected error creating gate:", err)
	}
}

func enableGate(t testing.TB, tier *feature.Tier, family, gate, collection string, volume float64, open bool) {
	t.Helper()

	if err := tier.EnableGate(family, gate, collection, volume, open); err != nil {
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
