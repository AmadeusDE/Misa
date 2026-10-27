// Package webadmin is the device and session administration CLI shared by the
// programs that host a webui.Server (ughbot, misa).
package webadmin

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/AmadeusDE/Misa/internal/webui"
)

const callTimeout = 10 * time.Second

// Env says where the device store is and how this program names itself.
type Env struct {
	Admin   *webui.Admin
	Out     io.Writer
	In      io.Reader
	Program string // command prefix shown in usage, e.g. "ughbot devices"
	Issuer  string // authenticator app label
	KeyPath string // CLI device key used by enroll
}

// Usage lists the device commands under the env's program name.
func (e Env) Usage() string {
	return fmt.Sprintf(`usage: %s <command>

  list                    enrolled devices
  revoke <id|name>        revoke a device and end its sessions
  rename <id|name> <new>  rename a device
  totp-init               create the enrollment secret (prints QR, otpauth URI and secret)
  totp-reset              replace the secret (devices and sessions stay)
  enroll [-name n] <url>  enroll this machine through the web UI at url
`, e.Program)
}

// Devices runs one device command: list, revoke, rename, totp-init, totp-reset or enroll.
func Devices(e Env, args []string) error {
	if len(args) == 0 {
		fmt.Fprint(e.Out, e.Usage())
		return fmt.Errorf("devices: missing command")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return listDevices(e.Admin, e.Out)
	case "revoke":
		if len(rest) != 1 {
			return fmt.Errorf("usage: %s revoke <id|name>", e.Program)
		}
		return e.Admin.RevokeDevice(rest[0])
	case "rename":
		if len(rest) != 2 {
			return fmt.Errorf("usage: %s rename <id|name> <new name>", e.Program)
		}
		return e.Admin.RenameDevice(rest[0], rest[1])
	case "totp-init":
		return e.printTOTP(e.Admin.TOTPInit)
	case "totp-reset":
		return e.printTOTP(e.Admin.TOTPReset)
	case "enroll":
		return e.enroll(rest)
	}
	return fmt.Errorf("devices: unknown command %q", sub)
}

// Sessions runs one session command: list or kill <prefix>.
func Sessions(e Env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: sessions list|kill <prefix>")
	}
	switch args[0] {
	case "list":
		rows, err := e.Admin.Sessions()
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(e.Out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "SESSION\tDEVICE\tIP\tCREATED\tEXPIRES")
		for _, r := range rows {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.DeviceName, r.IP, stamp(r.Created), stamp(r.Expires))
		}
		return tw.Flush()
	case "kill":
		if len(args) != 2 {
			return fmt.Errorf("usage: sessions kill <prefix>")
		}
		return e.Admin.KillSession(args[1])
	}
	return fmt.Errorf("sessions: unknown command %q", args[0])
}

func (e Env) printTOTP(make func(issuer, account string) (string, string, error)) error {
	secret, uri, err := make(e.Issuer, e.Issuer)
	if err != nil {
		return err
	}
	if qrText, err := webui.QRText(uri); err == nil {
		fmt.Fprintf(e.Out, "scan with an authenticator app:\n\n%s\n", qrText)
	}
	fmt.Fprintf(e.Out, "otpauth URI: %s\nsecret (base32): %s\n", uri, secret)
	fmt.Fprintf(e.Out, "add it to an authenticator app, then enroll a device at /enroll or with `%s enroll <url>`\n", e.Program)
	return nil
}

func listDevices(admin *webui.Admin, out io.Writer) error {
	devices, err := admin.Devices()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tCREATED\tLAST SEEN\tSTATE")
	for _, d := range devices {
		state := "active"
		if d.Revoked {
			state = "revoked"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", d.ID, d.Name, stamp(d.Created), stamp(d.LastSeen), state)
	}
	return tw.Flush()
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func (e Env) enroll(args []string) error {
	fs := flag.NewFlagSet("enroll", flag.ContinueOnError)
	host, _ := os.Hostname()
	name := fs.String("name", host, "device name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: %s enroll [-name n] <url>", e.Program)
	}
	key, err := webui.LoadOrCreateKey(e.KeyPath)
	if err != nil {
		return err
	}
	c, err := webui.NewClient(fs.Arg(0), key)
	if err != nil {
		return err
	}
	fmt.Fprint(e.Out, "TOTP code: ")
	code, err := bufio.NewReader(e.In).ReadString('\n')
	if err != nil && code == "" {
		return fmt.Errorf("no TOTP code read: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	if err := c.Enroll(ctx, strings.TrimSpace(code), *name); err != nil {
		return err
	}
	if err := c.Login(ctx); err != nil {
		return fmt.Errorf("enrolled but login failed: %w", err)
	}
	fmt.Fprintf(e.Out, "enrolled %q as device %s (key %s)\n", *name, c.DeviceID, e.KeyPath)
	return nil
}
