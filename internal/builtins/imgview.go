package builtins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/imgview"
)

var imgviewSpec = mustSpec("(thumbnail)(renderer):(start):r(recursive)")

// ErrImgviewInfo indicates that help or version output was requested.
var ErrImgviewInfo = errors.New("help or version requested")

func init() { Register(Command{Name: "imgview", Run: runImgview}) }

// ImgviewViewer parses command arguments and constructs a viewer. dir is the
// shell's working directory, which may differ from the process working directory.
func ImgviewViewer(args []string, dir string) (*imgview.Viewer, error) {
	flags, err := imgviewSpec.ParseArgs(args)
	if err != nil {
		return nil, err
	}
	if flags.HelpRequested || flags.VersionRequested {
		return nil, ErrImgviewInfo
	}
	cfg := config.LoadImageViewer()
	renderer := cfg.Renderer
	if value, ok := flags.String("renderer"); ok {
		renderer = value
	}
	switch renderer {
	case "auto", "halfblock", "kitty":
	default:
		return nil, fmt.Errorf("unknown renderer %q (choose auto, halfblock, or kitty)", renderer)
	}
	if len(flags.Operands) == 0 {
		return nil, errors.New("at least one FILE is required")
	}
	paths := make([]string, len(flags.Operands))
	for i, path := range flags.Operands {
		if !filepath.IsAbs(path) && dir != "" {
			path = filepath.Join(dir, path)
		}
		paths[i] = path
	}
	collection, err := imgview.CollectPaths(paths, imgview.CollectOptions{
		Recursive:         flags.Bool("r"),
		MaxFiles:          cfg.MaxImages,
		MaxDepth:          cfg.MaxDirectoryDepth,
		MaxEntriesScanned: cfg.MaxDirectoryEntries,
	})
	if err != nil {
		return nil, err
	}
	thumbnail := flags.Bool("thumbnail") || cfg.InitialMode == "thumbnails"
	viewer, err := imgview.NewConfigured(collection.Paths, thumbnail, renderer, cfg)
	if err != nil {
		return nil, err
	}
	if start, ok := flags.String("start"); ok {
		if start != "" && !filepath.IsAbs(start) && dir != "" {
			start = filepath.Join(dir, start)
		}
		if err := viewer.SetStartPath(start); err != nil {
			return nil, err
		}
	}
	notice := collection.Warning
	if os.Getenv("MISA_IMGVIEW_EMBEDDED") == "1" && os.Getenv("MISA_IMGVIEW_LIMITED") == "1" {
		notice = imgview.BrowserCollectionWarning
	}
	viewer.SetNotice(notice)
	// A configured cell override wins; otherwise measure the host terminal,
	// which is how the Misa GUI reports its font metrics to this process.
	viewer.SetCellSize(imgview.CellPixelsFor(cfg, imgview.MeasureCellPixels()))
	return viewer, nil
}

func runImgview(ctx *Context) int {
	if ctx.CmdName == "" {
		copy := *ctx
		copy.CmdName = "imgview"
		ctx = &copy
	}
	flags, err := imgviewSpec.ParseArgs(ctx.Args)
	if err == nil && HandleHelpVersion(ctx, flags, imgviewSpec) {
		if flags.HelpRequested {
			fmt.Fprintln(ctx.Stdout, "\nOpen FILEs, or the images inside DIRs, in order. -r/--recursive also descends\ninto subdirectories; hidden entries are skipped. --start FILE selects an item\nin that collection without reordering it. --renderer accepts auto,\nhalfblock, or kitty (forced pixels); auto probes the terminal.")
		}
		return 0
	}
	viewer, err := ImgviewViewer(ctx.Args, ctx.Dir)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "imgview:", err)
		return 1
	}
	if err := imgview.Run(viewer); err != nil {
		fmt.Fprintln(ctx.Stderr, "imgview:", err)
		return 1
	}
	return 0
}
