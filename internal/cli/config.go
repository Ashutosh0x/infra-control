package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Configuration resolution.
//
// A team running infractl in CI ends up repeating the same flags on every
// invocation: which state file, which snapshot, which severity gates a build.
// Putting those in a file that lives with the code means the settings are
// reviewed like the code is, and a pipeline definition stays short enough to
// read.
//
// Precedence, highest first:
//
//  1. An explicit command-line flag
//  2. An environment variable, INFRACTL_ prefixed
//  3. The configuration file
//  4. The flag's own default
//
// This ordering is what people expect from kubectl, docker, and gh, and it is
// the order that lets a pipeline override one setting without restating the
// rest of the file.

// configEnvPrefix namespaces the environment variables.
const configEnvPrefix = "INFRACTL"

// configBaseName is the file name looked for when none is given, without its
// extension. Viper resolves the extension, so .yaml, .yml, .json, and .toml all
// work without the user being told which to pick.
const configBaseName = ".infractl"

// loadConfig reads the configuration file and binds it, along with the
// environment, to the flags of the command about to run.
//
// A missing file is not an error. Most invocations have no config file, and
// requiring one would make the simple case worse to serve the complex one. A
// file that exists but cannot be parsed is an error, because the user clearly
// meant it to be used.
func loadConfig(cmd *cobra.Command) error {
	v := viper.New()
	v.SetEnvPrefix(configEnvPrefix)

	// Flag names are kebab-case and environment variables are SCREAMING_SNAKE,
	// so --min-severity is read from INFRACTL_MIN_SEVERITY.
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()

	if err := locateConfig(v); err != nil {
		return err
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) || os.IsNotExist(err) {
			// No file anywhere in the search path. Environment variables still
			// apply, so binding continues.
			return bindFlags(cmd, v)
		}
		return failf(ExitConfig, "read config file %s: %w", v.ConfigFileUsed(), err)
	}

	if err := applyProfile(v); err != nil {
		return err
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "infractl: using config %s\n", v.ConfigFileUsed())
	}
	return bindFlags(cmd, v)
}

// locateConfig points viper at either the explicitly requested file or the
// default search path.
func locateConfig(v *viper.Viper) error {
	if cfgFile != "" {
		// An explicitly named file that is absent is an error: the user asked
		// for it, so silently continuing without it would be wrong.
		if _, err := os.Stat(cfgFile); err != nil {
			return failf(ExitConfig, "config file not found: %s", cfgFile)
		}
		v.SetConfigFile(cfgFile)
		return nil
	}

	v.SetConfigName(configBaseName)

	// The working directory first, so a repository's own settings win over a
	// developer's personal defaults.
	v.AddConfigPath(".")
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(home)
		v.AddConfigPath(filepath.Join(home, ".config", "infractl"))
	}
	return nil
}

// applyProfile promotes a named profile's settings to the top level.
//
// A profile groups settings for one environment, so a single file can hold
// staging and production without either leaking into the other:
//
//	min-severity: info
//	profiles:
//	  production:
//	    min-severity: high
//	    fail-on: high
func applyProfile(v *viper.Viper) error {
	if profile == "" {
		return nil
	}

	key := "profiles." + profile
	if !v.IsSet(key) {
		available := "none"
		if profiles := v.GetStringMap("profiles"); len(profiles) > 0 {
			names := make([]string, 0, len(profiles))
			for name := range profiles {
				names = append(names, name)
			}
			available = strings.Join(names, ", ")
		}
		return failf(ExitConfig,
			"profile %q is not defined in %s.\n  Profiles available: %s",
			profile, v.ConfigFileUsed(), available)
	}

	for name, value := range v.GetStringMap(key) {
		v.Set(name, value)
	}
	return nil
}

// bindFlags copies resolved values onto flags the user did not set explicitly.
//
// Only unset flags are touched, which is what keeps a command-line flag ahead
// of the file: cobra records whether a flag was Changed, and a flag the user
// typed is left exactly as they typed it.
func bindFlags(cmd *cobra.Command, v *viper.Viper) error {
	var bindErr error

	visit := func(flag *pflag.Flag) {
		if bindErr != nil || flag.Changed {
			return
		}
		if !v.IsSet(flag.Name) {
			return
		}

		value := v.GetString(flag.Name)
		if err := cmd.Flags().Set(flag.Name, value); err != nil {
			bindErr = failf(ExitConfig,
				"config value for %q is not valid: %w", flag.Name, err)
		}
	}

	// Both sets are visited because a subcommand's own flags and the inherited
	// persistent flags are configured the same way from the user's side.
	cmd.Flags().VisitAll(visit)
	cmd.InheritedFlags().VisitAll(visit)

	return bindErr
}
