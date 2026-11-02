package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// ImageViewerConfig is the imageviewer.yaml configuration. All limits are
// bounded by Validate; sizes in MiB use binary megabytes.
type ImageViewerConfig struct {
	OpenMode               string   `yaml:"open_mode"`
	Renderer               string   `yaml:"renderer"`
	InitialMode            string   `yaml:"initial_mode"`
	Fit                    string   `yaml:"fit"`
	ThumbnailSize          int      `yaml:"thumbnail_size"`
	Preview                bool     `yaml:"preview"`
	PreviewDelayMS         int      `yaml:"preview_delay_ms"`
	AnimationAutoplay      bool     `yaml:"animation_autoplay"`
	PreviewAnimation       bool     `yaml:"preview_animation"`
	AnimationLoops         string   `yaml:"animation_loops"`
	AnimationFPS           int      `yaml:"animation_fps"`
	HalfblockAnimationFPS  int      `yaml:"halfblock_animation_fps"`
	PreviewAnimationFPS    int      `yaml:"preview_animation_fps"`
	AnimationMinDelayMS    int      `yaml:"animation_min_delay_ms"`
	GIFShortDelayMS        int      `yaml:"gif_short_delay_ms"`
	AnimationCacheMiB      int      `yaml:"animation_cache_mib"`
	AnimationMaxFrames     int      `yaml:"animation_max_frames"`
	AnimationMetadataMiB   int      `yaml:"animation_metadata_mib"`
	AnimationTransportMiBS int      `yaml:"animation_transport_mib_s"`
	CacheMiB               int      `yaml:"cache_mib"`
	MaxFileMiB             int      `yaml:"max_file_mib"`
	MaxPixels              int      `yaml:"max_pixels"`
	MaxImages              int      `yaml:"max_images"`
	MaxDirectoryDepth      int      `yaml:"max_directory_depth"`
	MaxDirectoryEntries    int      `yaml:"max_directory_entries"`
	ExternalCommand        []string `yaml:"external_command"`
}

// DefaultImageViewer returns platform-specific defaults.
func DefaultImageViewer() ImageViewerConfig {
	c := ImageViewerConfig{OpenMode: "builtin", Renderer: "auto", InitialMode: "single", Fit: "shrink", ThumbnailSize: 128, Preview: true, PreviewDelayMS: 120, AnimationAutoplay: true, AnimationLoops: "file", AnimationFPS: 30, HalfblockAnimationFPS: 10, PreviewAnimationFPS: 10, AnimationMinDelayMS: 20, GIFShortDelayMS: 100, AnimationCacheMiB: 64, AnimationMaxFrames: 10000, AnimationMetadataMiB: 8, AnimationTransportMiBS: 16, CacheMiB: 128, MaxFileMiB: 64, MaxPixels: 32000000, MaxImages: 10000, MaxDirectoryDepth: 32, MaxDirectoryEntries: 200000}
	if runtime.GOOS == "android" {
		c.AnimationFPS, c.HalfblockAnimationFPS, c.PreviewAnimationFPS = 20, 8, 8
		c.AnimationCacheMiB, c.AnimationMaxFrames, c.AnimationMetadataMiB, c.AnimationTransportMiBS = 24, 2000, 2, 4
	}
	return c
}

// Validate clamps numeric limits and replaces invalid enum values with defaults.
// It returns descriptions of changes, suitable for diagnostics.
func (c *ImageViewerConfig) Validate() []string {
	d := DefaultImageViewer()
	var warnings []string
	enum := func(name string, p *string, fallback string, choices ...string) {
		for _, choice := range choices {
			if *p == choice {
				return
			}
		}
		warnings = append(warnings, fmt.Sprintf("%s: invalid value %q; using %q", name, *p, fallback))
		*p = fallback
	}
	clamp := func(name string, p *int, min, max int) {
		old := *p
		if *p < min {
			*p = min
		}
		if *p > max {
			*p = max
		}
		if old != *p {
			warnings = append(warnings, fmt.Sprintf("%s: %d out of range; using %d", name, old, *p))
		}
	}
	enum("open_mode", &c.OpenMode, d.OpenMode, "builtin", "external")
	enum("renderer", &c.Renderer, d.Renderer, "auto", "kitty", "halfblock")
	enum("initial_mode", &c.InitialMode, d.InitialMode, "single", "thumbnails")
	enum("fit", &c.Fit, d.Fit, "shrink", "contain", "width", "height", "actual")
	enum("animation_loops", &c.AnimationLoops, d.AnimationLoops, "file", "once", "infinite")
	clamp("thumbnail_size", &c.ThumbnailSize, 32, 1024)
	clamp("preview_delay_ms", &c.PreviewDelayMS, 0, 60000)
	clamp("animation_fps", &c.AnimationFPS, 1, 120)
	clamp("halfblock_animation_fps", &c.HalfblockAnimationFPS, 1, 120)
	clamp("preview_animation_fps", &c.PreviewAnimationFPS, 1, 120)
	clamp("animation_min_delay_ms", &c.AnimationMinDelayMS, 1, 60000)
	clamp("gif_short_delay_ms", &c.GIFShortDelayMS, 1, 60000)
	clamp("cache_mib", &c.CacheMiB, 1, 1024)
	clamp("animation_cache_mib", &c.AnimationCacheMiB, 1, c.CacheMiB)
	clamp("animation_max_frames", &c.AnimationMaxFrames, 1, 10000)
	clamp("animation_metadata_mib", &c.AnimationMetadataMiB, 1, 64)
	clamp("animation_transport_mib_s", &c.AnimationTransportMiBS, 1, 256)
	clamp("max_file_mib", &c.MaxFileMiB, 1, 1024)
	clamp("max_pixels", &c.MaxPixels, 1, 32000000)
	clamp("max_images", &c.MaxImages, 1, 1000000)
	clamp("max_directory_depth", &c.MaxDirectoryDepth, 1, 256)
	clamp("max_directory_entries", &c.MaxDirectoryEntries, 1, 10000000)
	return warnings
}

// DefaultImageViewerPath returns the user config path used for a new file.
func DefaultImageViewerPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "imageviewer.yaml"
	}
	return filepath.Join(dir, "misa", "imageviewer.yaml")
}

// LoadImageViewer loads the first config in the standard search path. Missing
// fields retain defaults, including true booleans explicitly set to false.
func LoadImageViewer() ImageViewerConfig {
	defaults := DefaultImageViewer()
	path := findConfigFile("imageviewer.yaml")
	if path == "" {
		return defaults
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaults
	}
	c := defaults
	if err := yaml.Unmarshal(data, &c); err != nil {
		fmt.Fprintf(os.Stderr, "misa: image viewer config parse error in %s: %v\n", path, err)
		return defaults
	}
	for _, warning := range c.Validate() {
		fmt.Fprintf(os.Stderr, "misa: image viewer config %s: %s\n", path, warning)
	}
	return c
}

// SaveImageViewer writes the existing config, or creates one in the user directory.
func SaveImageViewer(c ImageViewerConfig) error {
	path := findConfigFile("imageviewer.yaml")
	if path == "" {
		path = DefaultImageViewerPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	}
	c.Validate()
	data, err := yaml.Marshal(&c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
