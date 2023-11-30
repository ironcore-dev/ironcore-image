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

package tag

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag source-image[:tag] target-image[:tag]",
		Short: "Tag a local image with a given name (and optional tag).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			srcImage := args[0]
			tgtImage := args[1]
			return Run(ctx, storeFactory, srcImage, tgtImage)
		},
	}

	return cmd
}

func Run(ctx context.Context, storeFactory common.StoreFactory, srcImage, tgtImage string) error {
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("could not create store: %w", err)
	}

	desc, err := common.FuzzyResolveRef(ctx, s, srcImage)
	if err != nil {
		return fmt.Errorf("error resolving source: %w", err)
	}

	if err := s.Tag(ctx, desc, tgtImage); err != nil {
		return fmt.Errorf("error tagging image: %w", err)
	}

	fmt.Println("Successfully tagged", tgtImage, "with", desc)
	return nil
}
