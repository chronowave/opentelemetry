package main

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

const (
	dataDir  = "chronowave.dir"
	httpPort = "chronowave.http"
)

type conf struct {
	dir  string
	port int
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

	return &conf{
		dir:  v.GetString(dataDir),
		port: v.GetInt(httpPort),
	}
}
