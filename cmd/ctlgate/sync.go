package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/segmentio/cli/human"
	"github.com/segmentio/feature"
)

type source struct{ human.Path }
type target struct{ human.Path }

type syncConfig struct {
}

func sync(config syncConfig, source source, target target) error {
	return withDB(string(source.Path), func(db *sql.DB) error {
		path, err := filepath.Abs(string(target.Path))
		if err != nil {
			return err
		}

		tmp, err := ioutil.TempDir(filepath.Dir(path), "ctlgate.*")
		if err != nil {
			return err
		}

		tiers := make(tiersCache)
		defer tiers.close()

		log.Printf("syncing ctlstore gates to %s", tmp)
		if err := syncGates(feature.MountPoint(tmp), db, tiers); err != nil {
			return err
		}

		log.Printf("syncing ctlstore collections to %s", tmp)
		if err := syncCollections(feature.MountPoint(tmp), db, tiers); err != nil {
			return err
		}

		link := tmp + ".symlink"
		if err := os.Symlink(filepath.Base(tmp), link); err != nil {
			return err
		}

		prev, err := os.Readlink(string(target.Path))
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		}

		log.Printf("updating target to point to new state at %s", target.Path)
		if err := os.Rename(link, string(target.Path)); err != nil {
			return err
		}

		if prev != "" {
			log.Printf("removing previous state at %s", prev)
			if err := os.RemoveAll(prev); err != nil {
				return err
			}
		}

		return nil
	})
}

func syncGates(path feature.MountPoint, db *sql.DB, tiers tiersCache) error {
	rows, err := db.Query(`select family, name, id_type, tier_list_id, rollout, salt, open from flagon2___gates where archived = '0'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var (
		family      string
		name        string
		idType      string
		tierListID  string
		rollout     string
		salt        int
		open        int
		numGates    int
		tierVolumes map[string]float64
	)

	for rows.Next() {
		if err := rows.Scan(&family, &name, &idType, &tierListID, &rollout, &salt, &open); err != nil {
			return err
		}

		if err := json.Unmarshal([]byte(rollout), &tierVolumes); err != nil {
			return fmt.Errorf("decoding rollout map of %s/%s/%s: %w", tierListID, family, name, err)
		}

		if open != 0 {
			for tier := range tierVolumes {
				tierVolumes[tier] = 100
			}
		}

		for tier, volume := range tierVolumes {
			if err := createGate(path, tierListID, tier, family, name, idType, uint32(salt), volume/100, tiers); err != nil {
				return err
			}
			numGates++
		}

		for tier := range tierVolumes {
			delete(tierVolumes, tier)
		}
	}

	log.Printf("successfully synced %d gates in %d tiers", numGates, len(tiers))
	return nil
}

func createGate(path feature.MountPoint, group, tier, family, name, collection string, salt uint32, volume float64, tiers tiersCache) error {
	t, err := tiers.open(path, group, tier)
	if err != nil {
		return err
	}
	defer t.Close()

	if err := t.CreateGate(family, name, collection, salt); err != nil {
		return err
	}

	return t.EnableGate(family, name, collection, volume)
}

func syncCollections(path feature.MountPoint, db *sql.DB, tiers tiersCache) error {
	rows, err := db.Query(`select tier_list_id, id_type, id, tier_id from flagon2___tier_assignments_v2`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var (
		tierListID  string
		idType      string
		id          string
		tierID      string
		numIDs      int
		collections = map[groupTierCollection]*feature.Collection{}
	)

	for rows.Next() {
		if err := rows.Scan(&tierListID, &idType, &id, &tierID); err != nil {
			return err
		}

		key := groupTierCollection{
			group:      tierListID,
			tier:       tierID,
			collection: idType,
		}

		col := collections[key]
		if col == nil {
			t, err := tiers.open(path, tierListID, tierID)
			if err != nil {
				return err
			}
			c, err := t.CreateCollection(idType)
			if err != nil {
				return err
			}
			col = c
			collections[key] = c
		}

		if err := col.Add(id); err != nil {
			return err
		}

		numIDs++
	}

	for _, col := range collections {
		if err := col.Close(); err != nil {
			return err
		}
	}

	log.Printf("successfully synced %d ids from %d collections in %d tiers", numIDs, len(collections), len(tiers))
	return nil
}

type groupTier struct {
	group string
	tier  string
}

type groupTierCollection struct {
	group      string
	tier       string
	collection string
}

type tiersCache map[groupTier]*feature.Tier

func (c tiersCache) open(path feature.MountPoint, group, tier string) (*feature.Tier, error) {
	t, ok := c[groupTier{group: group, tier: tier}]
	if ok {
		return t, nil
	}
	t, err := path.CreateTier(group, tier)
	if err != nil {
		return nil, err
	}
	c[groupTier{group: group, tier: tier}] = t
	return t, nil
}

func (c tiersCache) close() {
	for _, t := range c {
		t.Close()
	}
}
