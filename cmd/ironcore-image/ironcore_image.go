// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package ironcoreimage

import (
	"github.com/ironcore-dev/ironcore-image/cmd/build"
	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/ironcore-dev/ironcore-image/cmd/delete"
	"github.com/ironcore-dev/ironcore-image/cmd/inspect"
	"github.com/ironcore-dev/ironcore-image/cmd/list"
	"github.com/ironcore-dev/ironcore-image/cmd/pull"
	"github.com/ironcore-dev/ironcore-image/cmd/push"
	"github.com/ironcore-dev/ironcore-image/cmd/tag"
	"github.com/ironcore-dev/ironcore-image/cmd/url"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var (
		storePath   string
		configPaths []string
	)

	var (
		storeFactory           = common.DefaultStoreFactory(&storePath)
		registryFactory        = common.DefaultRemoteRegistryFactory(configPaths)
		requestResolverFactory = common.DefaultRequestResolverFactory(configPaths)
	)

	cmd := &cobra.Command{
		Use:   "ironcore-image",
		Short: "Commands to interface with ironcore images.",
	}

	cmd.AddCommand(
		build.Command(storeFactory),
		push.Command(storeFactory, registryFactory),
		pull.Command(storeFactory, registryFactory),
		tag.Command(storeFactory),
		list.Command(storeFactory),
		inspect.Command(storeFactory),
		delete.Command(storeFactory),
		url.Command(requestResolverFactory),
	)

	cmd.PersistentFlags().StringVar(&storePath, common.RecommendedStorePathFlagName, common.DefaultStorePath, common.RecommendedStorePathFlagUsage)
	cmd.PersistentFlags().StringSliceVar(&configPaths, common.RecommendedDockerConfigPathsFlagName, nil, common.RecommendedDockerConfigPathsFlagName)

	return cmd
}
