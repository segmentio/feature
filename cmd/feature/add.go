package main

import "github.com/segmentio/feature"

type addConfig struct {
	commonConfig
}

func add(config addConfig, group group, tier tier, collection collection, ids []id) error {
	return config.mount(func(path feature.MountPoint) error {
		t, err := path.OpenTier(string(group), string(tier))
		if err != nil {
			return err
		}
		defer t.Close()

		c, err := t.CreateCollection(string(collection))
		if err != nil {
			return err
		}
		defer c.Close()

		for _, id := range ids {
			if err := c.Add(string(id)); err != nil {
				return err
			}
		}

		return c.Sync()
	})
}
