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

package inspect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	onmetalimage "github.com/onmetal/onmetal-image"
	ociimage "github.com/onmetal/onmetal-image/oci/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/onmetal/onmetal-image/cmd/common"

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
	Descriptor ocispec.Descriptor  `json:"descriptor"`
	Manifest   ocispec.Manifest    `json:"manifest"`
	Config     onmetalimage.Config `json:"config"`
}

func readImageConfig(ctx context.Context, img ociimage.Image) (*onmetalimage.Config, error) {
	configLayer, err := img.Config(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting config layer: %w", err)
	}

	rc, err := configLayer.Content(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting config content: %w", err)
	}
	defer func() { _ = rc.Close() }()

	config := &onmetalimage.Config{}
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
