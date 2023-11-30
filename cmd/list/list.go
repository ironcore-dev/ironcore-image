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

package list

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/distribution/reference"
	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/ironcore-dev/ironcore-image/oci/descriptormatcher"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all images that are available locally.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return Run(ctx, storeFactory)
		},
	}

	return cmd
}

func Run(ctx context.Context, storeFactory common.StoreFactory) error {
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("could not create layout: %w", err)
	}

	descs, err := s.Layout().Indexer().List(ctx, descriptormatcher.MediaTypes(ocispec.MediaTypeImageManifest))
	if err != nil {
		return fmt.Errorf("error listing images: %w", err)
	}

	// Sort for some deterministic output
	sort.Slice(descs, func(i, j int) bool {
		r1 := descs[i].Annotations[ocispec.AnnotationRefName]
		r2 := descs[i].Annotations[ocispec.AnnotationRefName]
		if res := strings.Compare(r1, r2); res != 0 {
			return res < 0
		}
		return descs[i].Digest > descs[j].Digest
	})

	w := tabwriter.NewWriter(os.Stdout, 12, 0, 1, ' ', 0)
	_, _ = fmt.Fprintln(w, "REPOSITORY\tTAG\tIMAGE ID")
	for _, item := range descs {
		repo := "<none>"
		tag := "<none>"
		r := item.Annotations[ocispec.AnnotationRefName]
		if ref, err := reference.ParseNamed(r); err == nil {
			repo = ref.Name()
			if tagged, ok := ref.(reference.Tagged); ok {
				tag = tagged.Tag()
			}
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", repo, tag, item.Digest.Encoded()[:12])
	}
	return w.Flush()
}
