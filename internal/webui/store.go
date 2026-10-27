// Package webui is a generic authenticated web shell: device keys, TOTP
// enrollment, challenge login, sessions, CSRF, layout and mount points.
package webui

import "time"

type Device struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	PubKey   []byte    `json:"pubkey"`
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"last_seen"`
	Revoked  bool      `json:"revoked"`
}

type TOTPState struct {
	Secret      string `json:"secret"`
	LastCounter int64  `json:"last_counter"`
}

type Session struct {
	ID       string    `json:"id"`
	DeviceID string    `json:"device_id"`
	CSRF     string    `json:"csrf"`
	IP       string    `json:"ip"`
	Created  time.Time `json:"created"`
	Expires  time.Time `json:"expires"`
}

type Event struct {
	At       time.Time `json:"at"`
	Kind     string    `json:"kind"`
	DeviceID string    `json:"device_id,omitempty"`
	Name     string    `json:"name,omitempty"`
	IP       string    `json:"ip,omitempty"`
	Detail   string    `json:"detail,omitempty"`
}

// Notice is a banner shown to every device until it dismisses it.
type Notice struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	At        time.Time `json:"at"`
	Dismissed []string  `json:"dismissed,omitempty"`
}

type State struct {
	Devices  []Device  `json:"devices"`
	TOTP     TOTPState `json:"totp"`
	Sessions []Session `json:"sessions"`
	Events   []Event   `json:"events"`
	Notices  []Notice  `json:"notices"`
}

const maxEvents = 1000

// DeviceStore persists State. View callbacks must not retain or mutate the
// state; an Update callback that returns an error leaves the state unchanged.
type DeviceStore interface {
	View(fn func(*State)) error
	Update(fn func(*State) error) error
}

func (s *State) device(id string) *Device {
	for i := range s.Devices {
		if s.Devices[i].ID == id {
			return &s.Devices[i]
		}
	}
	return nil
}

func (s *State) log(e Event) {
	s.Events = append(s.Events, e)
	if len(s.Events) > maxEvents {
		s.Events = append([]Event(nil), s.Events[len(s.Events)-maxEvents:]...)
	}
}

func (s *State) dropSessions(match func(Session) bool) int {
	kept := s.Sessions[:0]
	dropped := 0
	for _, x := range s.Sessions {
		if match(x) {
			dropped++
			continue
		}
		kept = append(kept, x)
	}
	s.Sessions = kept
	return dropped
}
