package main

import (
	"fmt"
	"os"

	"github.com/segmentio/cli/human"
	"github.com/segmentio/feature"
)

type enableConfig struct {
	commonConfig
}

func enable(config enableConfig, group group, tier tier, family family, gate gate, volume human.Ratio) error {
	return config.mount(func(path feature.MountPoint) error {
		g, err := path.OpenGate(string(family), string(gate))
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%s/%s: gate does not exist\n", family, gate)
			}
			return err
		}
		defer g.Close()

		t, err := path.OpenTier(string(group), string(tier))
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%s/%s: tier does not exist\n", group, tier)
			}
			return err
		}
		defer t.Close()

		return t.EnableGate(string(family), string(gate), float64(volume))
	})
}
