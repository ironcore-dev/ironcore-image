// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

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

		config                                              Config
		kernelData, initramfsData, rootfsData, squashfsData []byte

		configLayer, kernelLayer, initramfsLayer, rootfsLayer, squashfsLayer image.Layer
		img, legacyImg                                                       image.Image
	)

	BeforeEach(func() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		DeferCleanup(cancel)

		config = Config{}
		kernelData = []byte("kernel")
		initramfsData = []byte("initramfs")
		rootfsData = []byte("rootfs")
		squashfsData = []byte("squashfs")

		c, err := imageutil.JSONValueLayer(config, imageutil.WithMediaType(ConfigMediaType))
		Expect(err).NotTo(HaveOccurred())
		configLayer = c
		kernelLayer = imageutil.BytesLayer(kernelData, imageutil.WithMediaType(KernelLayerMediaType))
		initramfsLayer = imageutil.BytesLayer(initramfsData, imageutil.WithMediaType(InitRAMFSLayerMediaType))
		rootfsLayer = imageutil.BytesLayer(rootfsData, imageutil.WithMediaType(RootFSLayerMediaType))
		squashfsLayer = imageutil.BytesLayer(squashfsData, imageutil.WithMediaType(SquashFSLayerMediaType))

		i, err := imageutil.NewBuilder(configLayer).
			Layers(kernelLayer, initramfsLayer, rootfsLayer, squashfsLayer).
			Complete()
		Expect(err).NotTo(HaveOccurred())
		img = i

		legacyC, err := imageutil.JSONValueLayer(config, imageutil.WithMediaType(LegacyConfigMediaType))
		Expect(err).NotTo(HaveOccurred())
		legacyConfigLayer := legacyC
		legacyKernelLayer := imageutil.BytesLayer(kernelData, imageutil.WithMediaType(LegacyKernelLayerMediaType))
		legacyInitramfsLayer := imageutil.BytesLayer(initramfsData, imageutil.WithMediaType(LegacyInitRAMFSLayerMediaType))
		legacyRootfsLayer := imageutil.BytesLayer(rootfsData, imageutil.WithMediaType(LegacyRootFSLayerMediaType))
		legacySquashfsLayer := imageutil.BytesLayer(squashfsData, imageutil.WithMediaType(LegacySquashFSLayerMediaType))

		legacyI, err := imageutil.NewBuilder(legacyConfigLayer).
			Layers(legacyKernelLayer, legacyInitramfsLayer, legacyRootfsLayer, legacySquashfsLayer).
			Complete()
		Expect(err).NotTo(HaveOccurred())
		legacyImg = legacyI
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
			Expect(imageutil.ReadLayerContent(ctx, res.SquashFS)).To(Equal(squashfsData))
		})

		It("should correctly resolve a legacy image", func() {
			By("resolving the image")
			res, err := ResolveImage(ctx, legacyImg)
			Expect(err).NotTo(HaveOccurred())

			By("inspecting the config")
			Expect(res.Config).To(Equal(config))

			By("inspecting the layers")
			Expect(imageutil.ReadLayerContent(ctx, res.Kernel)).To(Equal(kernelData))
			Expect(imageutil.ReadLayerContent(ctx, res.RootFS)).To(Equal(rootfsData))
			Expect(imageutil.ReadLayerContent(ctx, res.InitRAMFs)).To(Equal(initramfsData))
			Expect(imageutil.ReadLayerContent(ctx, res.SquashFS)).To(Equal(squashfsData))
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
	})
})
