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
	deleteTier(t, path, "group-A", "name-1")
}

func testMountPointDeleteGroupNotExist(t *testing.T, path feature.MountPoint) {
	deleteGroup(t, path, "group-A")
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
