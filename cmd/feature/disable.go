package main

import (
	"os"

	"github.com/segmentio/feature"
)

type disableConfig struct {
	commonConfig
}

func disable(config disableConfig, group group, tier tier, family family, gate gate, collection collection) error {
	return config.mount(func(path feature.MountPoint) error {
		t, err := path.OpenTier(string(group), string(tier))
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		defer t.Close()
		return t.EnableGate(string(family), string(gate), string(collection), 0, false)
	})
}
