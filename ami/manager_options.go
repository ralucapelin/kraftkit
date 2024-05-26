// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ami

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"kraftkit.sh/config"

	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	regtypes "github.com/docker/docker/api/types/registry"
	"github.com/mitchellh/go-homedir"
)

type AMIManagerOption func(context.Context, *amiManager) error

// fileExists returns true if the given path exists and is not a directory.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// defaultAuths uses the provided context to locate possible authentication
// values which can be used when speaking with remote registries.
func defaultAuths(ctx context.Context) (map[string]config.AuthConfig, error) {
	auths := make(map[string]config.AuthConfig)

	// Podman users may have their container registry auth configured in a
	// different location, that Docker packages aren't aware of.
	// If the Docker config file isn't found, we'll fallback to look where
	// Podman configures it, and parse that as a Docker auth config instead.

	// First, check $HOME/.docker/
	var home string
	var err error
	foundDockerConfig := false

	// If this is run in the context of GitHub actions, use an alternative path
	// for the $HOME.
	if os.Getenv("GITUB_ACTION") == "yes" {
		home = "/github/home"
	} else {
		home, err = homedir.Dir()
	}
	if err == nil {
		foundDockerConfig = fileExists(filepath.Join(home, ".docker/config.json"))
	}

	// If $HOME/.docker/config.json isn't found, check $DOCKER_CONFIG (if set)
	if !foundDockerConfig && os.Getenv("DOCKER_CONFIG") != "" {
		foundDockerConfig = fileExists(filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"))
	}

	// If either of those locations are found, load it using Docker's
	// config.Load, which may fail if the config can't be parsed.
	//
	// If neither was found, look for Podman's auth at
	// $XDG_RUNTIME_DIR/containers/auth.json and attempt to load it as a
	// Docker config.
	var cf *configfile.ConfigFile
	if foundDockerConfig {
		cf, err = cliconfig.Load(os.Getenv("DOCKER_CONFIG"))
		if err != nil {
			return nil, err
		}
	} else if f, err := os.Open(filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "containers/auth.json")); err == nil {
		defer f.Close()

		cf, err = cliconfig.LoadFromReader(f)
		if err != nil {
			return nil, err
		}
	}

	if cf != nil {
		for domain, cfg := range cf.AuthConfigs {
			if cfg.Username == "" && cfg.Password == "" {
				continue
			}
			auths[domain] = config.AuthConfig{
				Endpoint: cfg.ServerAddress,
				User:     cfg.Username,
				Token:    cfg.Password,
			}
		}
	}

	for domain, auth := range config.G[config.KraftKit](ctx).Auth {
		auths[domain] = auth
	}

	return auths, nil
}

// WithDefaultAuth uses the KraftKit-set configuration for authentication
// against remote registries.
func WithDefaultAuth() AMIManagerOption {
	return func(ctx context.Context, manager *amiManager) error {
		var err error

		manager.auths, err = defaultAuths(ctx)
		if err != nil {
			return err
		}

		return nil
	}
}

// WithRegistries sets the list of registries to use when making calls to
// non-canonically named OCI references.
func WithRegistries(registries ...string) AMIManagerOption {
	return func(ctx context.Context, manager *amiManager) error {
		manager.registries = registries
		return nil
	}
}

// WithDockerConfig sets the authentication configuration to use when making
// calls to authenticated registries.
func WithDockerConfig(auth regtypes.AuthConfig) AMIManagerOption {
	return func(ctx context.Context, manager *amiManager) error {
		if auth.ServerAddress == "" {
			return fmt.Errorf("cannot use auth config without server address")
		}

		if manager.auths == nil {
			manager.auths = make(map[string]config.AuthConfig, 1)
		}

		manager.auths[auth.ServerAddress] = config.AuthConfig{
			Endpoint: auth.ServerAddress,
			User:     auth.Username,
			Token:    auth.Password,
		}
		return nil
	}
}
