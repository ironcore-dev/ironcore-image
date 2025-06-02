// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

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

	descs, err := s.Layout().Indexer().List(ctx, descriptormatcher.MediaTypes(
		ocispec.MediaTypeImageManifest,
		ocispec.MediaTypeImageIndex,
	))

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
