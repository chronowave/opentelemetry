//+build mage

package main

import (
	// mage:import
	"github.com/grafana/grafana-plugin-sdk-go/build"
	"github.com/magefile/mage/sh"
)

// Hello prints a message (shows that you can define custom Mage targets).
func BuildUI() {
	err := sh.Run("yarn", "install")
	if err != nil {
		panic(err)
	}

	err = sh.Run("yarn", "build")
	if err != nil {
		panic(err)
	}
}

// Default configures the default target.
var Default = build.BuildAll
