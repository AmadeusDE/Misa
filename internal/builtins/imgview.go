package builtins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/imgview"
)

var imgviewSpec = mustSpec("(thumbnail)(renderer):")

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
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return nil, fmt.Errorf("%s: directories are not supported", flags.Operands[i])
		}
		paths[i] = path
	}
	thumbnail := flags.Bool("thumbnail") || cfg.InitialMode == "thumbnails"
	return imgview.NewConfigured(paths, thumbnail, renderer, cfg)
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
			fmt.Fprintln(ctx.Stdout, "\nOpen FILEs in order. --renderer accepts auto, halfblock, or kitty (falls back to halfblock).")
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
