package webui

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Admin is the shell-only recovery surface: no HTTP, no login.
type Admin struct {
	store DeviceStore
	now   func() time.Time
}

func NewAdmin(store DeviceStore, now func() time.Time) *Admin {
	if now == nil {
		now = time.Now
	}
	return &Admin{store: store, now: now}
}

func (a *Admin) Devices() ([]Device, error) {
	var out []Device
	err := a.store.View(func(st *State) { out = append(out, st.Devices...) })
	return out, err
}

func (a *Admin) Events(limit int) ([]Event, error) {
	var out []Event
	err := a.store.View(func(st *State) {
		ev := st.Events
		if limit > 0 && len(ev) > limit {
			ev = ev[len(ev)-limit:]
		}
		out = append(out, ev...)
	})
	return out, err
}

func findDevice(st *State, idOrName string) (*Device, error) {
	if d := st.device(idOrName); d != nil {
		return d, nil
	}
	var match *Device
	for i := range st.Devices {
		if st.Devices[i].Name == idOrName {
			if match != nil {
				return nil, fmt.Errorf("device name %q is ambiguous, use the id", idOrName)
			}
			match = &st.Devices[i]
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no device %q", idOrName)
	}
	return match, nil
}

// RevokeDevice revokes by id or unique name and ends its sessions at once.
func (a *Admin) RevokeDevice(idOrName string) error {
	return a.store.Update(func(st *State) error {
		d, err := findDevice(st, idOrName)
		if err != nil {
			return err
		}
		d.Revoked = true
		id := d.ID
		st.dropSessions(func(x Session) bool { return x.DeviceID == id })
		st.log(Event{At: a.now(), Kind: "revoke", DeviceID: id, Name: d.Name})
		return nil
	})
}

// validDeviceName: device names end up in admin terminals, so control
// characters (escape sequences) are refused.
func validDeviceName(name string) bool {
	return name != "" && len(name) <= 64 && utf8.ValidString(name) && !strings.ContainsFunc(name, unicode.IsControl)
}

func (a *Admin) RenameDevice(idOrName, newName string) error {
	newName = strings.TrimSpace(newName)
	if !validDeviceName(newName) {
		return fmt.Errorf("name must be 1-64 printable characters")
	}
	return a.store.Update(func(st *State) error {
		d, err := findDevice(st, idOrName)
		if err != nil {
			return err
		}
		d.Name = newName
		return nil
	})
}

// TOTPInit creates the enrollment secret; it refuses to replace an existing one.
func (a *Admin) TOTPInit(issuer, account string) (secret, uri string, err error) {
	return a.setTOTP(issuer, account, false)
}

// TOTPReset replaces the secret. Existing devices and sessions are untouched.
func (a *Admin) TOTPReset(issuer, account string) (secret, uri string, err error) {
	return a.setTOTP(issuer, account, true)
}

func (a *Admin) setTOTP(issuer, account string, replace bool) (string, string, error) {
	secret, uri, err := NewTOTPSecret(issuer, account)
	if err != nil {
		return "", "", err
	}
	err = a.store.Update(func(st *State) error {
		if st.TOTP.Secret != "" && !replace {
			return fmt.Errorf("TOTP already initialised, use totp-reset to replace it")
		}
		st.TOTP = TOTPState{Secret: secret, LastCounter: totpCounter(a.now())}
		kind := "totp_init"
		if replace {
			kind = "totp_reset"
		}
		st.log(Event{At: a.now(), Kind: kind})
		return nil
	})
	return secret, uri, err
}

type SessionInfoRow struct {
	ID         string // 12-character prefix, accepted by KillSession
	DeviceID   string
	DeviceName string
	IP         string
	Created    time.Time
	Expires    time.Time
}

func (a *Admin) Sessions() ([]SessionInfoRow, error) {
	var out []SessionInfoRow
	now := a.now()
	err := a.store.View(func(st *State) {
		for _, x := range st.Sessions {
			if !now.Before(x.Expires) {
				continue
			}
			row := SessionInfoRow{ID: x.ID[:12], DeviceID: x.DeviceID, IP: x.IP, Created: x.Created, Expires: x.Expires}
			if d := st.device(x.DeviceID); d != nil {
				row.DeviceName = d.Name
			}
			out = append(out, row)
		}
	})
	return out, err
}

// KillSession ends sessions whose id starts with prefix (at least 6 characters).
func (a *Admin) KillSession(prefix string) error {
	if len(prefix) < 6 {
		return fmt.Errorf("session id prefix must be at least 6 characters")
	}
	return a.store.Update(func(st *State) error {
		if n := st.dropSessions(func(x Session) bool { return strings.HasPrefix(x.ID, prefix) }); n == 0 {
			return fmt.Errorf("no session %q", prefix)
		}
		return nil
	})
}

// AdminDeviceID is the pseudo device on whose behalf the shell dismisses notices.
const AdminDeviceID = "admin"

// Notices lists enrollment notices the admin has not dismissed yet.
func (a *Admin) Notices() ([]Notice, error) {
	var out []Notice
	err := a.store.View(func(st *State) {
	next:
		for _, n := range st.Notices {
			for _, d := range n.Dismissed {
				if d == AdminDeviceID {
					continue next
				}
			}
			out = append(out, n)
		}
	})
	return out, err
}

// DismissNotice hides a notice from the admin list; browsers dismiss their own copy.
func (a *Admin) DismissNotice(id string) error {
	return a.store.Update(func(st *State) error {
		for i := range st.Notices {
			if st.Notices[i].ID == id {
				st.Notices[i].Dismissed = append(st.Notices[i].Dismissed, AdminDeviceID)
				return nil
			}
		}
		return fmt.Errorf("no notice %q", id)
	})
}
