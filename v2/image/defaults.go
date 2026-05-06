package image

const (
	// defaultConcurrency is the default concurrency to use.
	// The value is consistent with dockerd and containerd.
	defaultConcurrency int = 3

	// defaultMaxMetadataBytes is the default amount of bytes to use
	// for caching metadata.
	defaultMaxMetadataBytes int64 = 4 * 1024 * 1024 // 4 MiB

	// defaultMaxBytes is the default amount of bytes to use
	// for fetching content.
	defaultMaxBytes int64 = 4 * 1024 * 1024 // 4 MiB
)
