// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package ls

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/ironcore-dev/ironcore-image/v2/cli/ironcore-image/common"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/content/oci"
)

func Command(prov common.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Use: "ls",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			store, err := prov.Store(ctx)
			if err != nil {
				return err
			}

			return Run(ctx, store)
		},
	}

	return cmd
}

func Run(ctx context.Context, store *oci.Store) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	defer func() { _ = tw.Flush() }()

	_, _ = fmt.Fprintln(tw, "IMAGE\tID")
	return store.Tags(ctx, "", func(tags []string) error {
		for _, tag := range tags {
			desc, err := store.Resolve(ctx, tag)
			if err != nil {
				return fmt.Errorf("resolving tag %q: %w", tag, err)
			}

			_, _ = fmt.Fprintf(tw, "%s\t%s\n", tag, common.ShortEncoded(desc.Digest))
		}
		return nil
	})
}
