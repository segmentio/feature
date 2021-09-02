package main

import (
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"time"

	"github.com/segmentio/cli/human"
	"github.com/segmentio/feature"
)

type benchmarkConfig struct {
	commonConfig
	N          int        `flag:"-n,--count"    help:"Number of iteration taken by the benchmark"    default:"1000000"`
	CPUProfile human.Path `flag:"--cpu-profile" help:"Path where the CPU profile will be written"    default:"-"`
	MemProfile human.Path `flag:"--mem-profile" help:"Path where the memory profile will be written" default:"-"`
}

func benchmark(config benchmarkConfig, family family, gate gate, collection collection, id id) error {
	if config.CPUProfile != "" {
		f, err := os.OpenFile(string(config.CPUProfile), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	if config.MemProfile != "" {
		defer func() {
			f, err := os.OpenFile(string(config.MemProfile), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err == nil {
				defer f.Close()
				pprof.WriteHeapProfile(f)
			}
		}()
	}

	return config.mount(func(path feature.MountPoint) error {
		c, err := path.Load()
		if err != nil {
			return err
		}
		defer c.Close()
		start := time.Now()
		io.WriteString(os.Stdout, "BenchmarkGateOpen")

		for i := 0; i < config.N; i++ {
			c.GateOpen(string(family), string(gate), string(collection), string(id))
		}

		elapsed := time.Since(start)
		fmt.Printf("\t %d\t % 3d ns/op\n", config.N, int(float64(elapsed)/float64(config.N)))
		return nil
	})
}
