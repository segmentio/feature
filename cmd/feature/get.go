package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/segmentio/feature"
	"github.com/segmentio/fs"
)

type getConfig struct {
	commonConfig
	Watch bool `flag:"-w,--watch" help:"Runs the command then blocks waiting for changes and runs the command again"`
}

func (c *getConfig) mount(do func(feature.MountPoint) error) error {
	return c.commonConfig.mount(func(path feature.MountPoint) error {
		if !c.Watch {
			return do(path)
		}

		if err := path.Wait(context.Background()); err != nil {
			return err
		}

		notify := make(chan string)
		defer fs.Stop(notify)

		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

		for {
			if err := fs.Notify(notify, string(path)); err != nil {
				return err
			}
			if err := do(path); err != nil {
				log.Print(err)
			}
			select {
			case <-notify:
			case <-sigch:
				return nil
			}
		}
	})
}

type getTiersConfig struct {
	getConfig
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
	getConfig
	outputConfig
}

func getGates(config getGatesConfig, collection collection, id id) error {
	return config.mount(func(path feature.MountPoint) error {
		return config.table(func(w io.Writer) error {
			disabled := make(map[string]struct{})
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

					if err := feature.Scan(t.GatesDisabled(string(collection), string(id)), func(name string) error {
						disabled[name] = struct{}{}
						return nil
					}); err != nil {
						return err
					}

					return nil
				})
			}); err != nil {
				return err
			}

			for name := range disabled {
				delete(enabled, name)
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
