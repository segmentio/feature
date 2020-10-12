package feature_test

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/segmentio/feature"
)

func TestTier(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T, *feature.Tier)
	}{
		{
			scenario: "newly created tiers have no collections",
			function: testTierEmpty,
		},

		{
			scenario: "opening a collection which does not exist returns an error",
			function: testTierOpenCollectionNotExist,
		},

		{
			scenario: "collections created are exposed when listing collections",
			function: testTierCreateCollectionAndList,
		},

		{
			scenario: "collections deleted are not exposed when listing collections",
			function: testTierDeleteCollectionAndList,
		},

		{
			scenario: "ids added to a collection are exposed when listing ids",
			function: testTierCollectionAddAndList,
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
			test.function(t, tier)
		})
	}
}

func testTierEmpty(t *testing.T, tier *feature.Tier) {
	expectCollections(t, tier, []string{})
}

func testTierOpenCollectionNotExist(t *testing.T, tier *feature.Tier) {
	_, err := tier.OpenCollection("whatever")
	if err == nil || !os.IsNotExist(err) {
		t.Error("unexpected error:", err)
	}
}

func testTierCreateCollectionAndList(t *testing.T, tier *feature.Tier) {
	c1 := createCollection(t, tier, "collection-1")
	c2 := createCollection(t, tier, "collection-2")
	c3 := createCollection(t, tier, "collection-3")

	defer c1.Close()
	defer c2.Close()
	defer c3.Close()

	expectCollections(t, tier, []string{
		"collection-1",
		"collection-2",
		"collection-3",
	})
}

func testTierDeleteCollectionAndList(t *testing.T, tier *feature.Tier) {
	c1 := createCollection(t, tier, "collection-1")
	c2 := createCollection(t, tier, "collection-2")
	c3 := createCollection(t, tier, "collection-3")

	defer c1.Close()
	defer c2.Close()
	defer c3.Close()

	deleteCollection(t, tier, "collection-2")

	expectCollections(t, tier, []string{
		"collection-1",
		"collection-3",
	})
}

func testTierCollectionAddAndList(t *testing.T, tier *feature.Tier) {
	col := createCollection(t, tier, "collection")
	defer col.Close()

	populateCollection(t, col, []string{
		"id-1",
		"id-2",
		"id-3",
	})

	expectIDs(t, tier, "collection", []string{
		"id-1",
		"id-2",
		"id-3",
	})
}

func createCollection(t testing.TB, tier *feature.Tier, collection string) *feature.Collection {
	t.Helper()

	c, err := tier.CreateCollection(collection)
	if err != nil {
		t.Fatal(err)
	}

	return c
}

func deleteCollection(t testing.TB, tier *feature.Tier, collection string) {
	t.Helper()

	if err := tier.DeleteCollection(collection); err != nil {
		t.Error(err)
	}
}

func expectCollections(t testing.TB, tier *feature.Tier, collections []string) {
	t.Helper()
	found := readAll(t, tier.Collections())

	if !reflect.DeepEqual(found, collections) {
		t.Error("collections mismatch")
		t.Logf("want: %q", collections)
		t.Logf("got:  %q", found)
	}
}

func expectIDs(t testing.TB, tier *feature.Tier, collection string, ids []string) {
	t.Helper()
	found := readAll(t, tier.IDs(collection))

	if !reflect.DeepEqual(found, ids) {
		t.Error("ids mismatch")
		t.Logf("want: %q", ids)
		t.Logf("got:  %q", found)
	}
}

func populateCollection(t testing.TB, col *feature.Collection, ids []string) {
	t.Helper()

	for _, id := range ids {
		col.Add(id)
	}

	if err := col.Sync(); err != nil {
		t.Error(err)
	}
}
