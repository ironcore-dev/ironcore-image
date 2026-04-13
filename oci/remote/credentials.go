// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"context"
	"fmt"

	"oras.land/oras-go/v2/registry/remote/credentials"
)

// DockerCredentialFunc returns a credential function compatible with
// containerd's docker.ResolverOptions.Credentials and docker.WithAuthCreds.
// It reads Docker configuration from the given config paths.
// If no paths are provided, the default Docker configuration is used.
func DockerCredentialFunc(configPaths []string) (func(string) (string, string, error), error) {
	store, err := newCredentialStore(configPaths)
	if err != nil {
		return nil, fmt.Errorf("error creating credential store: %w", err)
	}
	credFunc := credentials.Credential(store)
	return func(hostport string) (string, string, error) {
		cred, err := credFunc(context.Background(), hostport)
		if err != nil {
			return "", "", err
		}
		if cred.Username == "" && cred.Password == "" {
			return "", cred.RefreshToken, nil
		}
		return cred.Username, cred.Password, nil
	}, nil
}

func newCredentialStore(configPaths []string) (credentials.Store, error) {
	if len(configPaths) == 0 {
		return credentials.NewStoreFromDocker(credentials.StoreOptions{})
	}
	if len(configPaths) == 1 {
		return credentials.NewStore(configPaths[0], credentials.StoreOptions{})
	}
	primary, err := credentials.NewStore(configPaths[0], credentials.StoreOptions{})
	if err != nil {
		return nil, fmt.Errorf("error creating primary credential store from %s: %w", configPaths[0], err)
	}
	var fallbacks []credentials.Store
	for _, p := range configPaths[1:] {
		s, err := credentials.NewStore(p, credentials.StoreOptions{})
		if err != nil {
			return nil, fmt.Errorf("error creating credential store from %s: %w", p, err)
		}
		fallbacks = append(fallbacks, s)
	}
	return credentials.NewStoreWithFallbacks(primary, fallbacks...), nil
}
