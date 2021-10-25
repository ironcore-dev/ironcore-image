package remote

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	ociimage "github.com/onmetal/onmetal-image/oci/image"
)

func WriteLayerToPusher(ctx context.Context, pusher remotes.Pusher, layer ociimage.Layer) error {
	w, err := pusher.Push(ctx, layer.Descriptor())
	if err != nil {
		return fmt.Errorf("error getting pusher: %w", err)
	}

	rc, err := layer.Content(ctx)
	if err != nil {
		_ = w.Close()
		return fmt.Errorf("error getting layer content: %w", err)
	}
	defer func() { _ = rc.Close() }()

	if err := content.Copy(ctx, w, rc, layer.Descriptor().Size, layer.Descriptor().Digest); err != nil {
		_ = w.Close()
		return fmt.Errorf("error copying layer: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("error closing writer: %w", err)
	}
	return nil
}

func WriteImageToPusher(ctx context.Context, pusher remotes.Pusher, img ociimage.Image) error {
	layers, err := ociimage.AsWriteLayers(ctx, img)
	if err != nil {
		return fmt.Errorf("error transforming image to write layers: %w", err)
	}

	for _, layer := range layers {
		if err := WriteLayerToPusher(ctx, pusher, layer); err != nil {
			return fmt.Errorf("error pushing layer %s: %w", layer.Descriptor().Digest, err)
		}
	}
	return nil
}
