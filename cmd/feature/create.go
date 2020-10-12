package main

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/segmentio/feature"
)

type createGateConfig struct {
	commonConfig
}

func createGate(config createGateConfig, family family, gate gate) error {
	return config.mount(func(path feature.MountPoint) error {
		var salt [4]byte

		if _, err := rand.Read(salt[:]); err != nil {
			return err
		}

		g, err := path.CreateGate(string(family), string(gate), binary.LittleEndian.Uint32(salt[:]))
		if err != nil {
			return err
		}
		g.Close()
		return nil
	})
}

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
