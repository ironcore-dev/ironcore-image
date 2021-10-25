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

package push

import (
	"context"
	"fmt"

	"github.com/onmetal/onmetal-image/cmd/common"

	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory, registryFactory common.RemoteRegistryFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push image[:tag]",
		Short: "Push a local image to a remote registry determined by the image name.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			return Run(ctx, storeFactory, registryFactory, name)
		},
	}

	return cmd
}

func Run(
	ctx context.Context,
	storeFactory common.StoreFactory,
	registryFactory common.RemoteRegistryFactory,
	ref string,
) error {
	store, err := storeFactory()
	if err != nil {
		return fmt.Errorf("error creating store: %w", err)
	}

	registry, err := registryFactory()
	if err != nil {
		return fmt.Errorf("error creating remote registry: %w", err)
	}

	img, err := store.Resolve(ctx, ref)
	if err != nil {
		return fmt.Errorf("error resolving ref %s: %w", ref, err)
	}

	if err := registry.Push(ctx, ref, img); err != nil {
		return fmt.Errorf("error pushing image to %s: %w", ref, err)
	}

	fmt.Println("Successfully pushed", ref, img.Descriptor().Digest.Encoded())
	return nil
}
