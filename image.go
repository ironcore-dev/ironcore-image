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

package onmetal_image

const (
	ConfigMediaType         = "application/vnd.onmetal.image.config.v1alpha1+json"
	RootFSLayerMediaType    = "application/vnd.onmetal.image.rootfs.v1alpha1.rootfs"
	InitRAMFSLayerMediaType = "application/vnd.onmetal.image.initramfs.v1alpha1.initramfs"
	KernelLayerMediaType    = "application/vnd.onmetal.image.vmlinuz.v1alpha1.vmlinuz"
)

type Config struct {
	CommandLine string `json:"commandLine,omitempty"`
}
