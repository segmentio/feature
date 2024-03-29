package feature

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	if err := mkdir(tier.pathTo("collections")); err != nil {
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

func (tier *Tier) Collections() *CollectionIter {
	return &CollectionIter{readdir(tier.pathTo("collections"))}
}

func (tier *Tier) IDs(collection string) *IDIter {
	return &IDIter{readfile(tier.collectionPath(collection))}
}

func (tier *Tier) Families() *FamilyIter {
	return &FamilyIter{readdir(tier.pathTo("gates"))}
}

func (tier *Tier) Gates(family string) *GateIter {
	return &GateIter{readdir(tier.familyPath(family))}
}

func (tier *Tier) GatesCreated(family, gate string) *GateCreatedIter {
	return &GateCreatedIter{readdir(tier.gatePath(family, gate))}
}

func (tier *Tier) GatesEnabled(collection, id string) *GateEnabledIter {
	id, families := tier.gates(collection, id)
	return &GateEnabledIter{
		path:       tier.path,
		families:   families,
		collection: collection,
		id:         id,
	}
}

func (tier *Tier) GatesDisabled(collection, id string) *GateDisabledIter {
	id, families := tier.gates(collection, id)
	return &GateDisabledIter{
		path:       tier.path,
		families:   families,
		collection: collection,
		id:         id,
	}
}

func (tier *Tier) gates(collection, id string) (string, dir) {
	it := tier.IDs(collection)
	defer it.Close()

	found := false
	for it.Next() {
		if it.Name() == id {
			found = true
			break
		}
	}

	if !found {
		id = ""
	}

	return id, readdir(tier.pathTo("gates"))
}

func (tier *Tier) CreateGate(family, name, collection string, salt uint32) error {
	if err := mkdir(tier.pathTo("gates")); err != nil {
		return err
	}
	if err := mkdir(tier.familyPath(family)); err != nil {
		return err
	}
	if err := mkdir(tier.gatePath(family, name)); err != nil {
		return err
	}
	return writeGate(tier.gateCollectionPath(family, name, collection), gate{
		salt: strconv.FormatUint(uint64(salt), 10),
	})
}

func (tier *Tier) EnableGate(family, name, collection string, volume float64, open bool) error {
	path := tier.gateCollectionPath(family, name, collection)
	g, err := readGate(path)
	if err != nil {
		return err
	}
	g.open, g.volume = open, volume
	return writeGate(path, g)
}

func (tier *Tier) ReadGate(family, name, collection string) (open bool, salt string, volume float64, err error) {
	g, err := readGate(tier.gateCollectionPath(family, name, collection))
	return g.open, g.salt, g.volume, err
}

func (tier *Tier) DeleteGate(family, name, collection string) error {
	return rmdir(tier.gateCollectionPath(family, name, collection))
}

func (tier *Tier) familyPath(family string) string {
	return filepath.Join(string(tier.path), tier.group, tier.name, "gates", family)
}

func (tier *Tier) gatePath(family, name string) string {
	return filepath.Join(string(tier.path), tier.group, tier.name, "gates", family, name)
}

func (tier *Tier) collectionPath(collection string) string {
	return filepath.Join(string(tier.path), tier.group, tier.name, "collections", collection)
}

func (tier *Tier) gateCollectionPath(family, name, collection string) string {
	return filepath.Join(string(tier.path), tier.group, tier.name, "gates", family, name, collection)
}

func (tier *Tier) pathTo(path string) string {
	return filepath.Join(string(tier.path), tier.group, tier.name, path)
}
