package main

import (
	"flag"
	"os"
	"sort"

	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
)

var (
	logger hclog.Logger
)

func init() {
	logger = hclog.New(&hclog.LoggerOptions{
		Name:       "chronowave",
		Level:      hclog.Warn, // Jaeger only captures >= Warn, so don't bother logging below Warn
		JSONFormat: true,
	})
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "A path to the chronowave plugin's configuration file")
	flag.Parse()

	environ := os.Environ()
	sort.Strings(environ)
	for _, env := range environ {
		logger.Warn(env)
	}

	conf := readConfig(configPath)
	rider := newWaveRider(logger, conf)
	defer rider.Close()

	grpc.Serve(&shared.PluginServices{
		Store: &cwPlugin{
			store: rider,
		},
	})
}
