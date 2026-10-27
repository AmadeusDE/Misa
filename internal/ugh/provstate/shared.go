package provstate

import "time"

func Get(provider string) (State, bool) { return std.Get(provider) }

func CheckRoute(provider, model, transport, accountID string) (State, bool) {
	return std.CheckRoute(provider, model, transport, accountID)
}

func InvalidateAccount(accountID string) { std.InvalidateAccount(accountID) }

func MarkExhausted(provider, accountID string, until time.Time, reason, source string) {
	std.MarkExhausted(provider, accountID, until, reason, source)
}

func MarkModelRejected(provider, model string, until time.Time, reason, source string) {
	std.MarkModelRejected(provider, model, until, reason, source)
}

func MarkOutage(provider string, until time.Time, reason, source string) {
	std.MarkOutage(provider, until, reason, source)
}

func Usable(provider string) bool    { return std.Usable(provider) }
func Exhausted(provider string) bool { return std.Exhausted(provider) }
func Mark(provider string, s State)  { std.Mark(provider, s) }
func Disable(provider string)        { std.Disable(provider) }
func Enable(provider string)         { std.Enable(provider) }
func Disabled(provider string) bool  { return std.Disabled(provider) }
func Clear(provider, kind string)    { std.Clear(provider, kind) }
func Summary() string                { return std.Summary() }
