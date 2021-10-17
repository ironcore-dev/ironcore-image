// Copyright 2021 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pull

import (
	"context"
	"fmt"

	"github.com/distribution/distribution/reference"

	"oras.land/oras-go/pkg/auth/docker"

	"github.com/onmetal/onmetal-image/client"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "pull ref",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ref := args[0]
			return Run(ctx, ref)
		},
	}

	return cmd
}

func Run(ctx context.Context, ref string) error {
	name, err := reference.ParseNamed(ref)
	if err != nil {
		return err
	}

	dockerClient, err := docker.NewClient()
	if err != nil {
		return err
	}

	c, err := client.New(client.WithAuthorizer(dockerClient))
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	desc, err := c.Pull(ctx, name)
	if err != nil {
		return fmt.Errorf("error pulling image: %w", err)
	}

	fmt.Println("Successfully pulled", ref, desc.Digest.Encoded())
	return nil
}
