// Copyright 2021 IronCore authors
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

	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/ironcore-dev/ironcore-image/oci/image"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory, registryFactory common.RemoteRegistryFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull image[:tag]",
		Short: "Pull an image from a remote registry determined by the image name.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ref := args[0]
			return Run(ctx, storeFactory, registryFactory, ref)
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
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("error creating store: %w", err)
	}

	registry, err := registryFactory()
	if err != nil {
		return fmt.Errorf("could not create remote registry: %w", err)
	}

	img, err := image.Copy(ctx, s, registry, ref)
	if err != nil {
		return fmt.Errorf("error pulling ref %s: %w", ref, err)
	}

	fmt.Println("Successfully pulled", ref, img.Descriptor().Digest.Encoded())
	return nil
}
