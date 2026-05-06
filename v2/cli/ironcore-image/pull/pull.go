// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pull

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/v2/cli/ironcore-image/common"
	"github.com/ironcore-dev/ironcore-image/v2/ui/btea"
	"github.com/ironcore-dev/ironcore-image/v2/ui/observable"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/errdef"
)

func Command(prov common.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Use: "pull NAME[:TAG]",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			name := args[0]

			store, err := prov.Store(ctx)
			if err != nil {
				return err
			}

			return Run(ctx, store, name)
		},
	}

	return cmd
}

func Run(ctx context.Context, store *oci.Store, name string) error {
	ref, err := common.ParseReferenceDefaultLatest(name)
	if err != nil {
		return err
	}

	ui := btea.New(btea.Options{
		TitleFunc: btea.DefaultTitleFunc,
		Verb:      "pulled",
	})
	defer ui.Stop()

	repo, err := common.RepositoryFor(ref)
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}

	observableStore := &observable.OCIStore{
		Store: store,
	}

	ui.MonitorPusher(observableStore)

	desc, err := oras.Copy(ctx, repo, ref.String(), observableStore, ref.String(), oras.CopyOptions{
		CopyGraphOptions: oras.CopyGraphOptions{
			OnCopySkipped: func(ctx context.Context, desc ocispec.Descriptor) error {
				ui.PushEvent(observable.PushStart{Descriptor: desc})
				ui.PushEvent(observable.PushDone{Descriptor: desc, Error: errdef.ErrAlreadyExists})
				return nil
			},
		},
	})
	if err != nil {
		return fmt.Errorf("copying image: %w", err)
	}

	if err := store.Tag(ctx, desc, ref.String()); err != nil {
		return fmt.Errorf("tagging image: %w", err)
	}
	return nil
}
