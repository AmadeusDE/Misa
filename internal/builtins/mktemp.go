package builtins

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const mktempSpec = "d(directory)p(dir):(tmpdir):?u(dry-run)tq(quiet)(silent)(suffix):?"

var mktempSpecParsed *Spec

func init() {
	var err error
	mktempSpecParsed, err = ParseSpec(mktempSpec)
	if err != nil {
		panic("builtins: bad mktemp spec: " + err.Error())
	}
	Register(Command{Name: "mktemp", Run: runMktemp})
}

func runMktemp(ctx *Context) int {
	flags, err := mktempSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "mktemp:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, mktempSpecParsed) {
		return 0
	}

	isDir := flags.Bool("d")
	dryRun := flags.Bool("u")
	quiet := flags.Bool("q") || flags.Bool("quiet") || flags.Bool("silent")

	suffixVal, hasSuffix := flags.String("suffix")

	pVal, hasP := flags.String("p")
	pValExplicit, _ := flags.String("dir")
	tdVal, hasTd := flags.ExplicitString("tmpdir")
	hasT := flags.Bool("t")

	if hasP && pVal == "" && pValExplicit != "" {
		pVal = pValExplicit
	}

	hasTmpDirFlag := hasP || hasTd || hasT

	tmpDirVal := ""
	if hasP && pVal != "" {
		tmpDirVal = resolvePath(ctx, pVal)
	} else if hasTd && tdVal != "" {
		tmpDirVal = resolvePath(ctx, tdVal)
	} else if env, ok := lookupEnv(ctx, "TMPDIR"); ok {
		if env == "" {
			if !quiet {
				fmt.Fprintln(ctx.Stderr, "mktemp: empty directory in TMPDIR")
			}
			return 1
		}
		tmpDirVal = resolvePath(ctx, env)
	} else {
		tmpDirVal = os.TempDir()
	}

	template := "tmp.XXXXXXXXXX"
	if len(flags.Operands) > 0 {
		template = flags.Operands[0]
	}
	if hasTmpDirFlag && filepath.IsAbs(template) {
		if !quiet {
			fmt.Fprintf(ctx.Stderr, "mktemp: invalid template, '%s'; with --tmpdir, it may not be absolute\n", template)
		}
		return 1
	}

	if hasSuffix {
		if template == "" || !strings.HasSuffix(template, "X") {
			if !quiet {
				fmt.Fprintf(ctx.Stderr, "mktemp: with --suffix, template '%s' must end in X\n", template)
			}
			return 1
		}
	}

	xCount := 0
	for i := len(template) - 1; i >= 0; i-- {
		if template[i] == 'X' {
			xCount++
		} else {
			break
		}
	}

	if xCount < 3 {
		fmt.Fprintf(ctx.Stderr, "mktemp: too few X's in template '%s'\n", template)
		return 1
	}

	xPos := len(template) - xCount
	prefix := template[:xPos]

	var baseDir string
	var displayTemplate string
	if strings.Contains(template, "/") {
		baseDir = resolvePath(ctx, filepath.Dir(template))
		prefix = filepath.Base(prefix)
		displayTemplate = template
	} else if hasTmpDirFlag || len(flags.Operands) == 0 {
		baseDir = tmpDirVal
		if tmpDirVal != "" {
			displayTemplate = filepath.Join(tmpDirVal, template)
		} else {
			displayTemplate = template
		}
	} else {
		baseDir = ctx.Dir
		displayTemplate = template
	}

	baseDir = resolvePath(ctx, baseDir)

	if st, serr := os.Stat(baseDir); serr != nil || !st.IsDir() {
		if !quiet {
			kind := "file"
			if isDir {
				kind = "directory"
			}
			fmt.Fprintf(ctx.Stderr, "mktemp: failed to create %s via template '%s': No such file or directory\n", kind, displayTemplate)
		}
		return 1
	}

	name, genErr := mktempGenerate(baseDir, prefix, xCount, suffixVal, isDir, dryRun)
	if genErr != nil {
		if !quiet {
			fmt.Fprintf(ctx.Stderr, "mktemp: %v\n", genErr)
		}
		return 1
	}
	if _, werr := fmt.Fprintln(ctx.Stdout, name); werr != nil {
		os.Remove(name)
		if !quiet {
			fmt.Fprintf(ctx.Stderr, "mktemp: %v\n", werr)
		}
		return 1
	}
	return 0
}

func lookupEnv(ctx *Context, key string) (string, bool) {
	if ctx.Env != nil {
		prefix := key + "="
		for _, e := range ctx.Env {
			if strings.HasPrefix(e, prefix) {
				return e[len(prefix):], true
			}
		}
		return "", false
	}
	return os.LookupEnv(key)
}

func mktempGenerate(dir, prefix string, xCount int, suffix string, isDir, dryRun bool) (string, error) {
	randBytes := make([]byte, (xCount+1)/2)

	for attempt := 0; attempt < 100; attempt++ {
		if _, err := rand.Read(randBytes); err != nil {
			return "", err
		}
		randHex := hex.EncodeToString(randBytes)[:xCount]
		name := filepath.Join(dir, prefix+randHex+suffix)

		if dryRun {
			return name, nil
		}

		if isDir {
			if err := os.Mkdir(name, 0700); err == nil {
				return name, nil
			} else if !os.IsExist(err) {
				return "", err
			}
		} else {
			f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err == nil {
				f.Close()
				return name, nil
			} else if !os.IsExist(err) {
				return "", err
			}
		}
	}
	return "", fmt.Errorf("failed to create unique name")
}
