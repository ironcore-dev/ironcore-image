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

package tag

import (
	"context"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/distribution/distribution/reference"

	"github.com/onmetal/onmetal-image/indexer"

	"github.com/onmetal/onmetal-image/client"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "tag source-image[:tag] target-image[:tag]",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			srcImage := args[0]
			tgtImage := args[1]
			return Run(ctx, srcImage, tgtImage)
		},
	}

	return cmd
}

func Run(ctx context.Context, srcImage, tgtImage string) error {
	c, err := client.New()
	if err != nil {
		return fmt.Errorf("could not create client: %w", err)
	}

	src, err := indexer.ResolveFuzzyRef(ctx, c.Indexer(), srcImage, indexer.WithMediaType(ocispec.MediaTypeImageManifest))
	if err != nil {
		return fmt.Errorf("error resolving source: %w", err)
	}

	ref, err := reference.ParseNamed(tgtImage)
	if err != nil {
		return fmt.Errorf("invalid reference %s: %w", tgtImage, err)
	}

	if _, err := c.Tag(ctx, src, ref); err != nil {
		return fmt.Errorf("error tagging image: %w", err)
	}

	fmt.Println("Successfully tagged", ref, "with", src.Digest.Encoded())
	return nil
}
