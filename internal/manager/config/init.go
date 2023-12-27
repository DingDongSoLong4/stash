package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"
)

type flagStruct struct {
	configFilePath string
	nobrowser      bool
}

var flags flagStruct

func init() {
	pflag.IP("host", net.IPv4(0, 0, 0, 0), "ip address for the host")
	pflag.Int("port", 9999, "port to serve from")
	pflag.StringVarP(&flags.configFilePath, "config", "c", "", "config file to use")
	pflag.BoolVar(&flags.nobrowser, "nobrowser", false, "Don't open a browser window after launch")
}

// Called at startup
func Initialize() (*Config, error) {
	cfg := &Config{
		main:      viper.New(),
		overrides: viper.New(),
	}

	cfg.initOverrides()

	err := cfg.initConfig()
	if err != nil {
		return nil, err
	}

	if cfg.isNewSystem {
		if cfg.Validate() == nil {
			// system has been initialised by the environment
			cfg.isNewSystem = false
		}
	}

	if !cfg.isNewSystem {
		cfg.setExistingSystemDefaults()

		err := cfg.SetInitialConfig()
		if err != nil {
			return nil, err
		}

		err = cfg.Write()
		if err != nil {
			return nil, err
		}

		err = cfg.Validate()
		if err != nil {
			return nil, err
		}
	}

	instance = cfg
	return instance, nil
}

// Called by tests to initialize an empty config
func InitializeEmpty() *Config {
	cfg := &Config{
		main:      viper.New(),
		overrides: viper.New(),
	}
	instance = cfg
	return instance
}

func bindEnv(v *viper.Viper, key string) {
	if err := v.BindEnv(key); err != nil {
		panic(fmt.Sprintf("unable to set environment key (%v): %v", key, err))
	}
}

func (i *Config) initOverrides() {
	v := i.overrides

	if err := v.BindPFlags(pflag.CommandLine); err != nil {
		logger.Infof("failed to bind flags: %v", err)
	}

	v.SetEnvPrefix("stash")     // will be uppercased automatically
	bindEnv(v, "host")          // STASH_HOST
	bindEnv(v, "port")          // STASH_PORT
	bindEnv(v, "external_host") // STASH_EXTERNAL_HOST
	bindEnv(v, "generated")     // STASH_GENERATED
	bindEnv(v, "metadata")      // STASH_METADATA
	bindEnv(v, "cache")         // STASH_CACHE
	bindEnv(v, "stash")         // STASH_STASH
}

func (i *Config) initConfig() error {
	v := i.main

	v.SetConfigType("yml")

	configFile := ""
	envConfigFile := os.Getenv("STASH_CONFIG_FILE")

	switch {
	case flags.configFilePath != "":
		configFile = flags.configFilePath
	case envConfigFile != "":
		configFile = envConfigFile
	default:
		// Look for config in the working directory and in $HOME/.stash
		paths := []string{
			".",
			filepath.Join(fsutil.GetHomeDirectory(), ".stash"),
		}
		configFile = fsutil.FindInPaths(paths, "config.yml")

		// if we haven't found a config file, we have a new system
		if configFile == "" {
			i.isNewSystem = true
			return nil
		}
	}

	v.SetConfigFile(configFile)

	// if the config file does not exist, we also have a new system
	if exists, _ := fsutil.FileExists(configFile); !exists {
		i.isNewSystem = true

		// ensure we can write to the file
		if err := fsutil.Touch(configFile); err != nil {
			return fmt.Errorf(`could not write to provided config path "%s": %v`, configFile, err)
		} else {
			// remove the file
			os.Remove(configFile)
		}

		return nil
	}

	err := v.ReadInConfig()
	if err != nil {
		return err
	}

	return nil
}
