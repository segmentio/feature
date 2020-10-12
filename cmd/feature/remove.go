package main

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/segmentio/feature"
)

type removeConfig struct {
	commonConfig
}

func remove(config removeConfig, group group, tier tier, collection collection, ids []id) error {
	return config.mount(func(path feature.MountPoint) error {
		t, err := path.OpenTier(string(group), string(tier))
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		defer t.Close()

		c, err := t.OpenCollection(string(collection))
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		defer c.Close()

		if len(ids) == 0 {
			return nil
		}

		index := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			index[string(id)] = struct{}{}
		}

		list := make([]string, 0, 100)
		if err := feature.Scan(c.IDs(), func(id string) error {
			if _, rm := index[id]; !rm {
				list = append(list, id)
			}
			return nil
		}); err != nil {
			return err
		}

		filePath := c.Path()
		f, err := ioutil.TempFile(filepath.Dir(filePath), "."+filepath.Base(filePath))
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		defer f.Close()
		w := bufio.NewWriter(f)

		for _, id := range list {
			w.WriteString(id)
			w.WriteString("\n")
		}

		if err := w.Flush(); err != nil {
			return err
		}

		return os.Rename(f.Name(), filePath)
	})
}
