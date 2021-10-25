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

package onmetalimage

import (
	"github.com/onmetal/onmetal-image/cmd/build"
	"github.com/onmetal/onmetal-image/cmd/common"
	"github.com/onmetal/onmetal-image/cmd/delete"
	"github.com/onmetal/onmetal-image/cmd/inspect"
	"github.com/onmetal/onmetal-image/cmd/list"
	"github.com/onmetal/onmetal-image/cmd/pull"
	"github.com/onmetal/onmetal-image/cmd/push"
	"github.com/onmetal/onmetal-image/cmd/tag"
	"github.com/onmetal/onmetal-image/cmd/url"
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
		Use:   "onmetal-image",
		Short: "Commands to interface with onmetal images.",
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
