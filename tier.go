package feature

import (
	"fmt"
	"os"
	"path/filepath"
)

type GroupIter struct{ dir }

func (it *GroupIter) Close() error { return it.close() }

func (it *GroupIter) Next() bool { return it.next() }

func (it *GroupIter) Name() string { return it.name() }

func (it *GroupIter) Tiers() *TierIter { return &TierIter{it.read()} }

type TierIter struct{ dir }

func (it *TierIter) Close() error { return it.close() }

func (it *TierIter) Next() bool { return it.next() }

func (it *TierIter) Name() string { return it.name() }

type Tier struct {
	path  MountPoint
	group string
	name  string
}

func (tier *Tier) Close() error {
	return nil
}

func (tier *Tier) String() string {
	return "/tiers/" + tier.group + "/" + tier.name
}

func (tier *Tier) Group() string {
	return tier.group
}

func (tier *Tier) Name() string {
	return tier.name
}

func (tier *Tier) CreateCollection(collection string) (*Collection, error) {
	if err := mkdir(tier.pathTo("ids")); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(tier.collectionPath(collection), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("creating collection %q: %w", collection, err)
	}
	return &Collection{file: f}, nil
}

func (tier *Tier) OpenCollection(collection string) (*Collection, error) {
	f, err := os.OpenFile(tier.collectionPath(collection), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			// Make a special case so the caller can use os.IsNotExist to test
			// the error.
			return nil, err
		}
		return nil, fmt.Errorf("opening collection %q: %w", collection, err)
	}
	return &Collection{file: f}, nil
}

func (tier *Tier) DeleteCollection(collection string) error {
	return unlink(tier.collectionPath(collection))
}

func (tier *Tier) Families() *FamilyIter {
	return &FamilyIter{readdir(tier.pathTo("gates"))}
}

func (tier *Tier) Gates(family string) *GateIter {
	return &GateIter{readdir(tier.familyPath(family))}
}

func (tier *Tier) Collections() *CollectionIter {
	return &CollectionIter{readdir(tier.pathTo("ids"))}
}

func (tier *Tier) IDs(collection string) *IDIter {
	return &IDIter{readfile(tier.collectionPath(collection))}
}

func (tier *Tier) GatesEnabled(collection, id string) *GateEnabledIter {
	return &GateEnabledIter{
		path:       tier.path,
		families:   tier.gatesEnabled(collection, id),
		collection: collection,
		id:         id,
	}
}

func (tier *Tier) gatesEnabled(collection, id string) dir {
	it := tier.IDs(collection)
	defer it.Close()

	for it.Next() {
		if it.Name() == id {
			return readdir(tier.pathTo("gates"))
		}
	}

	return dir{}
}

func (tier *Tier) EnableGate(family, name, collection string, volume float64) error {
	if err := mkdir(tier.pathTo("gates")); err != nil {
		return err
	}
	if err := mkdir(tier.familyPath(family)); err != nil {
		return err
	}
	if err := mkdir(tier.gatePath(family, name)); err != nil {
		return err
	}
	return writeFile(tier.gateCollectionPath(family, name, collection), func(f *os.File) error {
		_, err := fmt.Fprintf(f, "%g\n", volume)
		return err
	})
}

func (tier *Tier) DisableGateCollection(family, name, collection string) error {
	return unlink(tier.gateCollectionPath(family, name, collection))
}

func (tier *Tier) DisableGate(family, name string) error {
	return rmdir(tier.gatePath(family, name))
}

func (tier *Tier) DisableFamily(family string) error {
	return rmdir(tier.familyPath(family))
}

func (tier *Tier) DisableAll() error {
	return rmdir(tier.pathTo("gates"))
}

func (tier *Tier) ReadGate(family, name, collection string) (float64, error) {
	return readGate(tier.gateCollectionPath(family, name, collection))
}

func (tier *Tier) familyPath(family string) string {
	return filepath.Join(string(tier.path), "tiers", tier.group, tier.name, "gates", family)
}

func (tier *Tier) gatePath(family, name string) string {
	return filepath.Join(string(tier.path), "tiers", tier.group, tier.name, "gates", family, name)
}

func (tier *Tier) collectionPath(collection string) string {
	return filepath.Join(string(tier.path), "tiers", tier.group, tier.name, "ids", collection)
}

func (tier *Tier) gateCollectionPath(family, name, collection string) string {
	return filepath.Join(string(tier.path), "tiers", tier.group, tier.name, "gates", family, name, collection)
}

func (tier *Tier) pathTo(path string) string {
	return filepath.Join(string(tier.path), "tiers", tier.group, tier.name, path)
}
