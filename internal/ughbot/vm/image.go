package vm

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// fetchImage returns a local path for image, downloading a URL into dir once and checking it
// against the .SHA256 file published beside it when there is one.
func fetchImage(ctx context.Context, image, dir string, out io.Writer) (string, error) {
	if !strings.Contains(image, "://") {
		if _, err := os.Stat(image); err != nil {
			return "", err
		}
		return image, nil
	}
	dst := filepath.Join(dir, path.Base(image))
	if _, err := os.Stat(dst); err == nil {
		fmt.Fprintf(out, "image: using %s\n", dst)
		return dst, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	want, _ := publishedSum(ctx, image+".SHA256")
	fmt.Fprintf(out, "image: downloading %s\n", image)
	resp, err := get(ctx, image)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h, &progress{out: out, total: resp.ContentLength}), resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && want != "" && hex.EncodeToString(h.Sum(nil)) != want {
		err = fmt.Errorf("%s: checksum does not match the published SHA256", image)
	}
	if err != nil {
		os.Remove(tmp)
		return "", err
	}
	if want != "" {
		fmt.Fprintln(out, "image: checksum verified")
	}
	return dst, os.Rename(tmp, dst)
}

func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp, nil
}

// publishedSum reads the first hex digest from a sha256sum-style file.
func publishedSum(ctx context.Context, url string) (string, error) {
	resp, err := get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 1<<16))
	if sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) > 0 && len(f[0]) == 64 {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("%s: no digest", url)
}

type progress struct {
	out          io.Writer
	total, done  int64
	lastReported int64
}

func (p *progress) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.total > 0 && (p.done-p.lastReported)*10 >= p.total {
		p.lastReported = p.done
		fmt.Fprintf(p.out, "image: %d%%\n", p.done*100/p.total)
	}
	return len(b), nil
}
