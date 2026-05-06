package platforms

import (
	"fmt"
	"log/slog"
	"path"
	"runtime"
	"slices"
	"strings"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func Format(platform ocispec.Platform) string {
	if platform.OS == "" {
		return "unknown"
	}

	return path.Join(platform.OS, platform.Architecture, platform.Variant)
}

func Parse(platform string) (ocispec.Platform, error) {
	parts := strings.SplitN(platform, "/", 4)
	if len(parts) == 4 {
		return ocispec.Platform{}, fmt.Errorf("invalid platform %q", platform)
	}

	p := ocispec.Platform{OS: parts[0]}
	if len(parts) > 1 {
		p.Architecture = parts[1]
	}
	if len(parts) > 2 {
		p.Variant = parts[2]
	}
	return p, nil
}

func DefaultSpec() ocispec.Platform {
	return ocispec.Platform{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		// The Variant field will be empty if arch != ARM.
		Variant: cpuVariant(),
	}
}

// Present the ARM instruction set architecture, eg: v7, v8
// Don't use this value directly; call cpuVariant() instead.
var cpuVariantValue string

var cpuVariantOnce sync.Once

func cpuVariant() string {
	cpuVariantOnce.Do(func() {
		if slices.Contains([]string{"arm", "arm64"}, runtime.GOARCH) {
			var err error
			cpuVariantValue, err = "", nil
			if err != nil {
				slog.Error("Getting CPU variant for os", "OS", runtime.GOOS, "error", err)
			}
		}
	})
	return cpuVariantValue
}
