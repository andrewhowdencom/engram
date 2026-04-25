// Package config manages application configuration via Viper.
package config

import (
	"fmt"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
)

// appName is the XDG application name.
const appName = "engram"

// envPrefix is the prefix for environment variables.
const envPrefix = "ENGRAM"

// Load initializes Viper and reads configuration from file, environment, and defaults.
func Load(_ interface{}) error {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// Search XDG config directories.
	if xdg.ConfigHome != "" {
		v.AddConfigPath(xdg.ConfigHome)
	}
	for _, dir := range xdg.ConfigDirs {
		v.AddConfigPath(dir)
	}
	// Also look in a dedicated app subdirectory.
	v.AddConfigPath(fmt.Sprintf("%s/%s", xdg.ConfigHome, appName))

	// Environment variables.
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Defaults.
	v.SetDefault("log.level", "info")

	// Read file if present; ignore missing.
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	return nil
}
