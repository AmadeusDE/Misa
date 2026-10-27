package vm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const prepareTimeout = 45 * time.Minute

// Prepare builds the base image every code VM starts from: the cloud image, grown to
// disk_gb, booted once with cloud-init to add the login user, the pinned host key, the
// packages and the setup commands. Machines already running keep the base they started on.
func (m *Manager) Prepare(ctx context.Context, out io.Writer) error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return err
	}
	k, err := loadKeys(m.dir)
	if err != nil {
		return err
	}
	src, err := fetchImage(ctx, m.cfg.Image, filepath.Join(m.dir, "images"), out)
	if err != nil {
		return err
	}
	next := m.basePath() + ".new"
	defer os.Remove(next)
	fmt.Fprintln(out, "base: converting and growing to", m.cfg.DiskGB, "GB")
	if err := qemuImg("convert", "-O", "qcow2", src, next); err != nil {
		return err
	}
	if err := qemuImg("resize", next, strconv.Itoa(m.cfg.DiskGB)+"G"); err != nil {
		return err
	}
	sd, err := startSeed(func(report string) ([]byte, error) { return userData(m.cfg, k, os.Getuid(), report) })
	if err != nil {
		return err
	}
	defer sd.close()
	logPath := filepath.Join(m.dir, "prepare.log")
	_ = os.Remove(logPath)
	fmt.Fprintf(out, "base: booting with cloud-init (log: %s); installing packages takes a few minutes\n", logPath)
	q, err := startQEMU(qemuArgs(m.cfg, boot{disk: next, serial: logPath, smbios: sd.guestURL()}), logPath)
	if err != nil {
		return err
	}
	defer q.kill()
	timeout := time.NewTimer(prepareTimeout)
	defer timeout.Stop()
	select {
	case r := <-sd.result:
		if r != "done" {
			return fmt.Errorf("preparing failed inside the VM; see %s", logPath)
		}
	case <-q.done:
		return fmt.Errorf("the VM stopped before preparing finished (%v); see %s", q.err, logPath)
	case <-timeout.C:
		return fmt.Errorf("preparing took longer than %s; see %s", prepareTimeout, logPath)
	case <-ctx.Done():
		return ctx.Err()
	}
	fmt.Fprintln(out, "base: prepared, waiting for the VM to power off")
	select {
	case <-q.done:
	case <-time.After(3 * time.Minute):
		return errors.New("the VM did not power off after preparing")
	case <-ctx.Done():
		return ctx.Err()
	}
	meta, _ := json.Marshal(Base{Image: m.cfg.Image, Packages: m.cfg.Packages, Setup: m.cfg.Setup, At: time.Now()})
	if err := os.WriteFile(filepath.Join(m.dir, "base.json"), meta, 0o600); err != nil {
		return err
	}
	if err := os.Rename(next, m.basePath()); err != nil {
		return err
	}
	fmt.Fprintln(out, "base: ready at", m.basePath())
	return nil
}
