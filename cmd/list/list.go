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

package list

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/onmetal/onmetal-image/indexer"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/distribution/distribution/reference"

	"github.com/onmetal/onmetal-image/refutil"

	"github.com/onmetal/onmetal-image/client"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return Run(ctx)
		},
	}

	return cmd
}

func Run(ctx context.Context) error {
	c, err := client.New()
	if err != nil {
		return fmt.Errorf("could not create client: %w", err)
	}

	descs, err := c.Indexer().List(ctx, indexer.WithMediaType(ocispec.MediaTypeImageManifest))
	if err != nil {
		return fmt.Errorf("error listing images: %w", err)
	}

	// Sort for some deterministic output
	sort.Slice(descs, func(i, j int) bool {
		r1, _ := refutil.ReferenceFromDescriptor(descs[i])
		r2, _ := refutil.ReferenceFromDescriptor(descs[j])
		if r1 == nil && r2 == nil {
			return descs[i].Digest > descs[j].Digest
		}
		if r2 == nil {
			return true
		}
		if r1 == nil {
			return false
		}
		return descs[i].Digest > descs[j].Digest
	})

	w := tabwriter.NewWriter(os.Stdout, 12, 0, 1, ' ', 0)
	_, _ = fmt.Fprintln(w, "REPOSITORY\tTAG\tIMAGE ID")
	for _, item := range descs {
		repo := "<none>"
		tag := "<none>"
		r, _ := refutil.ReferenceFromDescriptor(item)
		if r != nil {
			repo = r.Name()
			if tagged, ok := r.(reference.Tagged); ok {
				tag = tagged.Tag()
			}
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", repo, tag, item.Digest.Encoded()[:12])
	}
	return w.Flush()
}
