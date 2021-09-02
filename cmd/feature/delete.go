package main

import "github.com/segmentio/feature"

type deleteTierConfig struct {
	commonConfig
}

func deleteTier(config deleteTierConfig, group group, tier tier) error {
	return config.mount(func(path feature.MountPoint) error {
		return path.DeleteTier(string(group), string(tier))
	})
}

type deleteGateConfig struct {
	commonConfig
}

func deleteGate(config deleteGateConfig, group group, tier tier, family family, gate gate, collection collection) error {
	return config.mount(func(path feature.MountPoint) error {
		t, err := path.OpenTier(string(group), string(tier))
		if err != nil {
			return err
		}
		defer t.Close()
		return t.DeleteGate(string(family), string(gate), string(collection))
	})
}
