// Copyright 2023 IronCore authors
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

package ironcoreimage_test

import (
	"context"

	. "github.com/ironcore-dev/ironcore-image"
	"github.com/ironcore-dev/ironcore-image/oci/image"
	"github.com/ironcore-dev/ironcore-image/oci/imageutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Image", func() {
	var (
		ctx context.Context

		config                                Config
		kernelData, initramfsData, rootfsData []byte

		configLayer, kernelLayer, initramfsLayer, rootfsLayer image.Layer
		img                                                   image.Image
	)

	BeforeEach(func() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		DeferCleanup(cancel)

		config = Config{}
		kernelData = []byte("kernel")
		initramfsData = []byte("initramfs")
		rootfsData = []byte("rootfs")

		c, err := imageutil.JSONValueLayer(config, imageutil.WithMediaType(ConfigMediaType))
		Expect(err).NotTo(HaveOccurred())
		configLayer = c
		kernelLayer = imageutil.BytesLayer(kernelData, imageutil.WithMediaType(KernelLayerMediaType))
		initramfsLayer = imageutil.BytesLayer(initramfsData, imageutil.WithMediaType(InitRAMFSLayerMediaType))
		rootfsLayer = imageutil.BytesLayer(rootfsData, imageutil.WithMediaType(RootFSLayerMediaType))

		i, err := imageutil.NewBuilder(configLayer).
			Layers(kernelLayer, initramfsLayer, rootfsLayer).
			Complete()
		Expect(err).NotTo(HaveOccurred())
		img = i
	})

	Describe("ResolveImage", func() {
		It("should correctly resolve the image", func() {
			By("resolving the image")
			res, err := ResolveImage(ctx, img)
			Expect(err).NotTo(HaveOccurred())

			By("inspecting the config")
			Expect(res.Config).To(Equal(config))

			By("inspecting the layers")
			Expect(imageutil.ReadLayerContent(ctx, res.Kernel)).To(Equal(kernelData))
			Expect(imageutil.ReadLayerContent(ctx, res.RootFS)).To(Equal(rootfsData))
			Expect(imageutil.ReadLayerContent(ctx, res.InitRAMFs)).To(Equal(initramfsData))
		})

		It("should error if the image contains invalid layers", func() {
			By("creating an image with an additional invalid layer")
			invalidLayer := imageutil.BytesLayer([]byte("invalid"))
			img, err := imageutil.NewBuilder(configLayer).
				Layers(kernelLayer, initramfsLayer, rootfsLayer, invalidLayer).
				Complete()
			Expect(err).NotTo(HaveOccurred())

			By("resolving the invalid image")
			_, err = ResolveImage(ctx, img)
			Expect(err).To(HaveOccurred())
		})

		It("should error if the image is missing layers", func() {
			By("creating an image with the kernel layer missing")
			img, err := imageutil.NewBuilder(configLayer).
				Layers(initramfsLayer, rootfsLayer).
				Complete()
			Expect(err).NotTo(HaveOccurred())

			By("resolving the invalid image")
			_, err = ResolveImage(ctx, img)
			Expect(err).To(HaveOccurred())
		})
	})
})
