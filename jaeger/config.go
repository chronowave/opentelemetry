package main

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	dataDir  = "chronowave.dir"
	dataTTL  = "chronowave.ttl"
	httpPort = "chronowave.http"
)

type conf struct {
	dir  string
	port int
	ttl  time.Duration
}

func readConfig(file string) *conf {
	v := viper.New()
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	if file != "" {
		v.SetConfigFile(file)

		err := v.ReadInConfig()
		if err != nil {
			logger.Error("failed to parse configuration file", "error", err)
			os.Exit(1)
		}
	}

	ttl, err := time.ParseDuration(v.GetString(dataTTL))
	if err != nil {
		logger.Error("failed to parse TTL duration, default to 3d", "ttl", v.GetString(dataTTL), "error", err)
		ttl = time.Hour * 3 * 24
	}

	return &conf{
		dir:  v.GetString(dataDir),
		port: v.GetInt(httpPort),
		ttl:  ttl,
	}
}
