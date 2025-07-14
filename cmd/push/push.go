// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/oci/content"

	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory, registryFactory common.RemoteRegistryFactory) *cobra.Command {
	var pushSubManifests bool

	cmd := &cobra.Command{
		Use:   "push image[:tag]",
		Short: "Push a local image to a remote registry determined by the image name.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			return Run(ctx, storeFactory, registryFactory, name, pushSubManifests)
		},
	}

	cmd.Flags().BoolVar(&pushSubManifests, "push-sub-manifests", false, "Push sub-manifests along with the index manifest.")
	return cmd
}

func Run(
	ctx context.Context,
	storeFactory common.StoreFactory,
	registryFactory common.RemoteRegistryFactory,
	ref string,
	pushSubManifests bool,
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

	// Check if the image is an index manifest
	if indexManifest, err := content.GetIndexManifest(ctx, img); err == nil && pushSubManifests {
		fmt.Println("Detected index manifest. Pushing sub-manifests...")

		for _, manifest := range indexManifest.Manifests {
			platform := manifest.Platform
			if platform == nil {
				return fmt.Errorf("platform information is missing for sub-manifest %s, cannot proceed", manifest.Digest)
			}
			archSuffix := "-" + platform.Architecture

			subRef := fmt.Sprintf("%s%s", ref, archSuffix)

			subImg, err := store.Resolve(ctx, manifest.Digest.String())
			if err != nil {
				return fmt.Errorf("error resolving sub-manifest %s: %w", manifest.Digest, err)
			}
			if err := registry.Push(ctx, subRef, subImg); err != nil {
				return fmt.Errorf("error pushing sub-manifest %s: %w", manifest.Digest, err)
			}
			fmt.Printf("Successfully pushed sub-manifest: %s\n", manifest.Digest)
		}

		if err := registry.Push(ctx, ref, img); err != nil {
			return fmt.Errorf("error pushing index manifest %s: %w", ref, err)
		}
		fmt.Println("Successfully pushed index manifest:", ref)
		return nil
	}

	if err := registry.Push(ctx, ref, img); err != nil {
		return fmt.Errorf("error pushing image to %s: %w", ref, err)
	}

	fmt.Println("Successfully pushed", ref, img.Descriptor().Digest.Encoded())
	return nil
}
