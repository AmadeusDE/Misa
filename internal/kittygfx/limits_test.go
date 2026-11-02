package kittygfx

import "testing"

func TestDefaultLimits(t *testing.T) {
	got := DefaultLimits()
	want := Limits{
		MaxWidth:         16384,
		MaxHeight:        16384,
		MaxImageBytes:    256 << 20,
		MaxTerminalBytes: 512 << 20,
		MaxImages:        256,
		MaxPlacements:    1024,
		MaxInFlightBytes: 360 << 20,
		MaxPayload:       8 << 10,
	}
	if got != want {
		t.Fatalf("DefaultLimits() = %+v, want %+v", got, want)
	}
}

func TestLimitsNormalize(t *testing.T) {
	def := DefaultLimits()
	tests := []struct {
		name string
		in   Limits
		want Limits
	}{
		{"zero", Limits{}, def},
		{"negative", Limits{MaxWidth: -1, MaxImages: -5}, def},
		{"partial", Limits{MaxWidth: 100, MaxPayload: 16}, Limits{
			MaxWidth: 100, MaxHeight: def.MaxHeight, MaxImageBytes: def.MaxImageBytes,
			MaxTerminalBytes: def.MaxTerminalBytes, MaxImages: def.MaxImages,
			MaxPlacements: def.MaxPlacements, MaxInFlightBytes: def.MaxInFlightBytes,
			MaxPayload: 16,
		}},
		{"complete", Limits{MaxWidth: 1, MaxHeight: 2, MaxImageBytes: 3, MaxTerminalBytes: 4, MaxImages: 5, MaxPlacements: 6, MaxInFlightBytes: 7, MaxPayload: 8},
			Limits{MaxWidth: 1, MaxHeight: 2, MaxImageBytes: 3, MaxTerminalBytes: 4, MaxImages: 5, MaxPlacements: 6, MaxInFlightBytes: 7, MaxPayload: 8}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Normalize(); got != tt.want {
				t.Fatalf("Normalize() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLimitsNormalizeIdempotent(t *testing.T) {
	once := Limits{MaxPayload: 4}.Normalize()
	if twice := once.Normalize(); twice != once {
		t.Fatalf("Normalize() not idempotent: %+v then %+v", once, twice)
	}
}
