package main

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/segmentio/feature"
)

type createTierConfig struct {
	commonConfig
}

func createTier(config createTierConfig, group group, tier tier) error {
	return config.mount(func(path feature.MountPoint) error {
		t, err := path.CreateTier(string(group), string(tier))
		if err != nil {
			return err
		}
		t.Close()
		return nil
	})
}

type createGateConfig struct {
	commonConfig
}

func createGate(config createGateConfig, group group, tier tier, family family, gate gate, collection collection) error {
	return config.mount(func(path feature.MountPoint) error {
		var salt [4]byte

		if _, err := rand.Read(salt[:]); err != nil {
			return err
		}

		t, err := path.OpenTier(string(group), string(tier))
		if err != nil {
			return err
		}
		defer t.Close()
		return t.CreateGate(string(family), string(gate), string(collection), binary.LittleEndian.Uint32(salt[:]))
	})
}
