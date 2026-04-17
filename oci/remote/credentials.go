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
// It reads Docker configuration from the given config path.
// If configPath is empty, the default Docker configuration is used.
func DockerCredentialFunc(configPath string) (func(string) (string, string, error), error) {
	var (
		store credentials.Store
		err   error
	)
	if configPath == "" {
		store, err = credentials.NewStoreFromDocker(credentials.StoreOptions{})
	} else {
		store, err = credentials.NewStore(configPath, credentials.StoreOptions{})
	}
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
