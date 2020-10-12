package main

import "github.com/segmentio/feature"

type deleteGateConfig struct {
	commonConfig
}

func deleteGate(config deleteGateConfig, family family, gate gate) error {
	return config.mount(func(path feature.MountPoint) error {
		return path.DeleteGate(string(family), string(gate))
	})
}

type deleteTierConfig struct {
	commonConfig
}

func deleteTier(config deleteTierConfig, group group, tier tier) error {
	return config.mount(func(path feature.MountPoint) error {
		return path.DeleteTier(string(group), string(tier))
	})
}
