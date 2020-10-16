package main

import (
	"fmt"
	"io"
	"os"

	"github.com/segmentio/feature"
)

type describeCollectionConfig struct {
	commonConfig
	outputConfig
	Group string `flag:"-g,--group" help:"Group to include in the collection description" default:"-"`
	Tier  string `flag:"-t,--tier"  help:"Tier to include in the collection description"  default:"-"`
}

func describeCollection(config describeCollectionConfig, collection collection) error {
	return config.mount(func(path feature.MountPoint) error {
		return config.buffered(func(w io.Writer) error {
			return feature.Scan(path.Groups(), func(group string) error {
				if config.Group != "" && config.Group != group {
					return nil
				}

				return feature.Scan(path.Tiers(group), func(tier string) error {
					if config.Tier != "" && config.Tier != tier {
						return nil
					}

					t, err := path.OpenTier(group, tier)
					if err != nil {
						return err
					}
					defer t.Close()

					return feature.Scan(t.IDs(string(collection)), func(id string) error {
						_, err := fmt.Fprintln(w, id)
						return err
					})
				})
			})
		})
	})
}

type describeTierConfig struct {
	commonConfig
	outputConfig
}

func describeTier(config describeTierConfig, group group, tier tier) error {
	return config.mount(func(path feature.MountPoint) error {
		return config.buffered(func(w io.Writer) error {
			t, err := path.OpenTier(string(group), string(tier))
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%s/%s: tier does not exist\n", group, tier)
				}
				return err
			}
			defer t.Close()

			fmt.Fprintf(w, "Group:\t%s\n", group)
			fmt.Fprintf(w, "Tier:\t%s\n", tier)
			fmt.Fprint(w, "Collections:\n")

			if err := feature.Scan(t.Collections(), func(collection string) error {
				_, err := fmt.Fprintf(w, " - %s\n", collection)
				return err
			}); err != nil {
				return err
			}

			fmt.Fprintf(w, "Gates:\n")

			if err := feature.Scan(t.Families(), func(family string) error {
				return feature.Scan(t.Gates(family), func(gate string) error {
					if _, err := fmt.Fprintf(w, "  %s/%s:\n", family, gate); err != nil {
						return err
					}

					return feature.Scan(t.GatesCreated(family, gate), func(collection string) error {
						open, _, volume, err := t.ReadGate(family, gate, collection)
						if err != nil {
							return err
						}
						_, err = fmt.Fprintf(w, "  - %s\t(%.0f%%, default: %s)\n", collection, volume*100, openFormat(open))
						return err
					})
				})
			}); err != nil {
				return err
			}

			return nil
		})
	})
}

type openFormat bool

func (open openFormat) Format(w fmt.State, _ rune) {
	if open {
		io.WriteString(w, "open")
	} else {
		io.WriteString(w, "close")
	}
}
