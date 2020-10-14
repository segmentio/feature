package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/segmentio/feature"
)

type getTiersConfig struct {
	commonConfig
	outputConfig
}

func getTiers(config getTiersConfig) error {
	return config.mount(func(path feature.MountPoint) error {
		return config.table(func(w io.Writer) error {
			fmt.Fprint(w, "GROUP\tTIER\tCOLLECTIONS\tFAMILIES\tGATES\n")
			return feature.Scan(path.Groups(), func(group string) error {
				return feature.Scan(path.Tiers(group), func(tier string) error {
					numCollections, numFamilies, numGates := 0, 0, 0

					t, err := path.OpenTier(group, tier)
					if err != nil {
						return err
					}
					defer t.Close()

					if err := feature.Scan(t.Collections(), func(string) error {
						numCollections++
						return nil
					}); err != nil {
						return err
					}

					if err := feature.Scan(t.Families(), func(family string) error {
						numFamilies++
						return feature.Scan(t.Gates(family), func(string) error {
							numGates++
							return nil
						})
					}); err != nil {
						return err
					}

					_, err = fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\n", group, tier, numCollections, numFamilies, numGates)
					return err
				})
			})
		})
	})
}

type getGatesConfig struct {
	commonConfig
	outputConfig
}

func getGates(config getGatesConfig, collection collection, id id) error {
	return config.mount(func(path feature.MountPoint) error {
		return config.table(func(w io.Writer) error {
			enabled := make(map[string]struct{})

			if err := feature.Scan(path.Groups(), func(group string) error {
				return feature.Scan(path.Tiers(group), func(tier string) error {
					t, err := path.OpenTier(group, tier)
					if err != nil {
						return err
					}
					defer t.Close()

					if err := feature.Scan(t.GatesEnabled(string(collection), string(id)), func(name string) error {
						enabled[name] = struct{}{}
						return nil
					}); err != nil {
						return err
					}

					return nil
				})
			}); err != nil {
				return err
			}

			if len(enabled) == 0 {
				return nil
			}

			list := make([]string, 0, len(enabled))
			for name := range enabled {
				list = append(list, name)
			}
			sort.Strings(list)

			fmt.Fprint(w, "FAMILY\tGATE\n")
			for _, name := range list {
				if _, err := fmt.Fprintln(w, strings.ReplaceAll(name, "/", "\t")); err != nil {
					return err
				}
			}

			return nil
		})
	})
}
