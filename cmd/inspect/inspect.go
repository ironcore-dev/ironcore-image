// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package inspect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	ironcoreimage "github.com/ironcore-dev/ironcore-image"
	"github.com/ironcore-dev/ironcore-image/cmd/common"
	ociimage "github.com/ironcore-dev/ironcore-image/oci/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect image[:tag]",
		Short: "Inspect a local image, i.e. get its manifest and some of its metadata.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			srcImage := args[0]
			return Run(ctx, storeFactory, srcImage)
		},
	}

	return cmd
}

type Output struct {
	Descriptor ocispec.Descriptor   `json:"descriptor"`
	Manifest   ocispec.Manifest     `json:"manifest"`
	Config     ironcoreimage.Config `json:"config"`
}

func readImageConfig(ctx context.Context, img ociimage.Image) (*ironcoreimage.Config, error) {
	configLayer, err := img.Config(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting config layer: %w", err)
	}

	rc, err := configLayer.Content(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting config content: %w", err)
	}
	defer func() { _ = rc.Close() }()

	config := &ironcoreimage.Config{}
	if err := json.NewDecoder(rc).Decode(config); err != nil {
		return nil, fmt.Errorf("error decoding config: %w", err)
	}
	return config, nil
}

func Run(ctx context.Context, storeFactory common.StoreFactory, srcImage string) error {
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("could not create store: %w", err)
	}

	ref, err := common.FuzzyResolveRef(ctx, s, srcImage)
	if err != nil {
		return fmt.Errorf("error resolving source: %w", err)
	}

	img, err := s.Resolve(ctx, ref)
	if err != nil {
		return fmt.Errorf("error getting image: %w", err)
	}

	manifest, err := img.Manifest(ctx)
	if err != nil {
		return fmt.Errorf("error reading image manifest: %w", err)
	}

	config, err := readImageConfig(ctx, img)
	if err != nil {
		return fmt.Errorf("error reading image config: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(Output{
		Descriptor: img.Descriptor(),
		Manifest:   *manifest,
		Config:     *config,
	})
}
