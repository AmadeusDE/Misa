// Package webdevtui is the Bubble Tea screen for managing web UI devices,
// sessions, the event log and enrollment notices. It knows nothing about
// webui or ughbot: hosts adapt their store or daemon client to Admin.
package webdevtui

import "time"

type Device struct {
	ID       string
	Name     string
	Created  time.Time
	LastSeen time.Time
	Revoked  bool
}

type Session struct {
	ID         string
	DeviceID   string
	DeviceName string
	IP         string
	Created    time.Time
	Expires    time.Time
}

type Event struct {
	At       time.Time
	Kind     string
	DeviceID string
	Name     string
	IP       string
	Detail   string
}

type Notice struct {
	ID   string
	Text string
	At   time.Time
}

type Admin interface {
	Devices() ([]Device, error)
	Revoke(idOrName string) error
	Rename(idOrName, newName string) error
	Sessions() ([]Session, error)
	KillSession(prefix string) error
	Events(limit int) ([]Event, error)
	Notices() ([]Notice, error)
	DismissNotice(id string) error
}
