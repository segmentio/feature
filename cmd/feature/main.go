package main

import (
	"bufio"
	"io"
	"os"
	"text/tabwriter"

	"github.com/segmentio/cli"
	"github.com/segmentio/cli/human"
	"github.com/segmentio/feature"
)

func main() {
	cli.Exec(cli.CommandSet{
		"benchmark": cli.Command(benchmark),
		"create": cli.CommandSet{
			"gate": cli.Command(createGate),
			"tier": cli.Command(createTier),
		},
		"delete": cli.CommandSet{
			"gate": cli.Command(deleteGate),
			"tier": cli.Command(deleteTier),
		},
		"get": cli.CommandSet{
			"gates": cli.Command(getGates),
			"tiers": cli.Command(getTiers),
		},
		"add":    cli.Command(add),
		"remove": cli.Command(remove),
		"describe": cli.CommandSet{
			"tier":       cli.Command(describeTier),
			"collection": cli.Command(describeCollection),
		},
		"enable":  cli.Command(enable),
		"disable": cli.Command(disable),
	})
}

type commonConfig struct {
	Path human.Path `flag:"-p,--path" help:"Path to the directory where the feature database is stored" default:"~/.feature"`
}

func (c *commonConfig) mount(do func(feature.MountPoint) error) error {
	if err := os.Mkdir(string(c.Path), 0755); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	p, err := feature.Mount(string(c.Path))
	if err != nil {
		return err
	}
	return do(p)
}

type outputConfig struct {
}

func (c *outputConfig) buffered(do func(io.Writer) error) error {
	bw := bufio.NewWriter(os.Stdout)
	defer bw.Flush()
	return do(bw)
}

func (c *outputConfig) table(do func(io.Writer) error) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer tw.Flush()
	return do(tw)
}

type family string

type group string

type collection string

type tier string

type gate string

type id string
