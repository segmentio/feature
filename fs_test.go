package feature_test

import (
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/segmentio/feature"
)

func TestMountPoint(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T, feature.MountPoint)
	}{
		{
			scenario: "opening a gate which does not exist returns an error",
			function: testMountPointOpenGateNotExist,
		},

		{
			scenario: "gates created are exposed when listing families and gates",
			function: testMountPointCreateGateAndList,
		},

		{
			scenario: "gates deleted are not exposed anymore when listing families and gates",
			function: testMountPointDeleteGateAndList,
		},

		{
			scenario: "deleting a gate which does not exist does nothing",
			function: testMountPointDeleteGateNotExist,
		},

		{
			scenario: "deleting a family which does not exist does nothing",
			function: testMountPointDeleteFamilyNotExist,
		},

		{
			scenario: "opening a tier which does not exist returns an error",
			function: testMountPointOpenTierNotExist,
		},

		{
			scenario: "tiers created are exposed when listing groups and tiers",
			function: testMountPointCreateTierAndList,
		},

		{
			scenario: "tiers deleted are not exposed anymore when listing groups and tiers",
			function: testMountPointDeleteTierAndList,
		},

		{
			scenario: "deleting a tier which does not exist does nothing",
			function: testMountPointDeleteTierNotExist,
		},

		{
			scenario: "deleting a group which does not exist does nothing",
			function: testMountPointDeleteGroupNotExist,
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			tmp, err := ioutil.TempDir("", "feature")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmp)
			p, err := feature.Mount(tmp)
			if err != nil {
				t.Fatal(err)
			}
			test.function(t, p)
		})
	}
}

func testMountPointOpenGateNotExist(t *testing.T, path feature.MountPoint) {
	_, err := path.OpenGate("hello", "world")
	if err == nil || !os.IsNotExist(err) {
		t.Error("unexpected error:", err)
	}
}

func testMountPointCreateGateAndList(t *testing.T, path feature.MountPoint) {
	g1 := createGate(t, path, "family-A", "name-1")
	g2 := createGate(t, path, "family-A", "name-2")
	g3 := createGate(t, path, "family-B", "name-3")

	defer g1.Close()
	defer g2.Close()
	defer g3.Close()

	expectFamilies(t, path, []string{
		"family-A",
		"family-B",
	})

	expectGates(t, path, "family-A", []string{
		"name-1",
		"name-2",
	})

	expectGates(t, path, "family-B", []string{
		"name-3",
	})
}

func testMountPointDeleteGateAndList(t *testing.T, path feature.MountPoint) {
	g1 := createGate(t, path, "family-A", "name-1")
	g2 := createGate(t, path, "family-A", "name-2")
	g3 := createGate(t, path, "family-B", "name-3")

	defer g1.Close()
	defer g2.Close()
	defer g3.Close()

	deleteGate(t, path, "family-A", "name-1")
	deleteGate(t, path, "family-B", "name-3")

	expectFamilies(t, path, []string{
		"family-A",
		"family-B",
	})

	expectGates(t, path, "family-A", []string{
		"name-2",
	})

	expectGates(t, path, "family-B", []string{})
}

func testMountPointDeleteGateNotExist(t *testing.T, path feature.MountPoint) {
	deleteGate(t, path, "family-A", "name-1")
}

func testMountPointDeleteFamilyNotExist(t *testing.T, path feature.MountPoint) {
	deleteFamily(t, path, "family-A")
}

func testMountPointOpenTierNotExist(t *testing.T, path feature.MountPoint) {
	_, err := path.OpenTier("hello", "world")
	if err == nil || !os.IsNotExist(err) {
		t.Error("unexpected error:", err)
	}
}

func testMountPointCreateTierAndList(t *testing.T, path feature.MountPoint) {
	t1 := createTier(t, path, "group-A", "name-1")
	t2 := createTier(t, path, "group-A", "name-2")
	t3 := createTier(t, path, "group-B", "name-3")

	defer t1.Close()
	defer t2.Close()
	defer t3.Close()

	expectGroups(t, path, []string{
		"group-A",
		"group-B",
	})

	expectTiers(t, path, "group-A", []string{
		"name-1",
		"name-2",
	})

	expectTiers(t, path, "group-B", []string{
		"name-3",
	})
}

func testMountPointDeleteTierAndList(t *testing.T, path feature.MountPoint) {
	t1 := createTier(t, path, "group-A", "name-1")
	t2 := createTier(t, path, "group-A", "name-2")
	t3 := createTier(t, path, "group-B", "name-3")

	defer t1.Close()
	defer t2.Close()
	defer t3.Close()

	deleteTier(t, path, "group-A", "name-1")
	deleteTier(t, path, "group-B", "name-3")

	expectGroups(t, path, []string{
		"group-A",
		"group-B",
	})

	expectTiers(t, path, "group-A", []string{
		"name-2",
	})

	expectTiers(t, path, "group-B", []string{})
}

func testMountPointDeleteTierNotExist(t *testing.T, path feature.MountPoint) {
	deleteGate(t, path, "group-A", "name-1")
}

func testMountPointDeleteGroupNotExist(t *testing.T, path feature.MountPoint) {
	deleteGroup(t, path, "group-A")
}

func createGate(t testing.TB, path feature.MountPoint, family, name string) *feature.Gate {
	t.Helper()

	g, err := path.CreateGate(family, name, 42)
	if err != nil {
		t.Fatal(err)
	}

	return g
}

func deleteFamily(t testing.TB, path feature.MountPoint, family string) {
	t.Helper()

	if err := path.DeleteFamily(family); err != nil {
		t.Error(err)
	}
}

func deleteGate(t testing.TB, path feature.MountPoint, family, name string) {
	t.Helper()

	if err := path.DeleteGate(family, name); err != nil {
		t.Error(err)
	}
}

func expectFamilies(t testing.TB, path feature.MountPoint, families []string) {
	t.Helper()
	found := readAll(t, path.Families())

	if !reflect.DeepEqual(found, families) {
		t.Error("families mismatch")
		t.Logf("want: %q", families)
		t.Logf("got:  %q", found)
	}
}

func expectGates(t testing.TB, path feature.MountPoint, family string, gates []string) {
	t.Helper()
	found := readAll(t, path.Gates(family))

	if !reflect.DeepEqual(found, gates) {
		t.Error("gates mismatch")
		t.Logf("want: %q", gates)
		t.Logf("got:  %q", found)
	}

	for _, name := range gates {
		g, err := path.OpenGate(family, name)
		if err != nil {
			t.Error(err)
			continue
		}
		if g.Family() != family {
			t.Errorf("gate family mismatch, want %q but got %q", family, g.Family())
		}
		if g.Name() != name {
			t.Errorf("gate name mismatch, want %q but got %q", name, g.Name())
		}
	}
}

func createTier(t testing.TB, path feature.MountPoint, group, name string) *feature.Tier {
	t.Helper()

	g, err := path.CreateTier(group, name)
	if err != nil {
		t.Fatal(err)
	}

	return g
}

func deleteGroup(t testing.TB, path feature.MountPoint, group string) {
	t.Helper()

	if err := path.DeleteGroup(group); err != nil {
		t.Error(err)
	}
}

func deleteTier(t testing.TB, path feature.MountPoint, group, name string) {
	t.Helper()

	if err := path.DeleteTier(group, name); err != nil {
		t.Error(err)
	}
}

func expectGroups(t testing.TB, path feature.MountPoint, groups []string) {
	t.Helper()
	found := readAll(t, path.Groups())

	if !reflect.DeepEqual(found, groups) {
		t.Error("groups mismatch")
		t.Logf("want: %q", groups)
		t.Logf("got:  %q", found)
	}
}

func expectTiers(t testing.TB, path feature.MountPoint, group string, tiers []string) {
	t.Helper()
	found := readAll(t, path.Tiers(group))

	if !reflect.DeepEqual(found, tiers) {
		t.Error("tiers mismatch")
		t.Logf("want: %q", tiers)
		t.Logf("got:  %q", found)
	}

	for _, name := range tiers {
		g, err := path.OpenTier(group, name)
		if err != nil {
			t.Error(err)
			continue
		}
		if g.Group() != group {
			t.Errorf("tier group mismatch, want %q but got %q", group, g.Group())
		}
		if g.Name() != name {
			t.Errorf("tier name mismatch, want %q but got %q", name, g.Name())
		}
	}
}

func readAll(t testing.TB, it feature.Iter) []string {
	values := []string{}
	if err := feature.Scan(it, func(v string) error {
		values = append(values, v)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(values)
	return values
}
