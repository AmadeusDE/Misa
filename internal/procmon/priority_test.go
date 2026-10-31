package procmon

import "testing"

func TestPriorityClassMapping(t *testing.T) {
	for _, c := range []PriorityClass{
		PriorityIdle, PriorityBelowNormal, PriorityNormal,
		PriorityAboveNormal, PriorityHigh, PriorityRealtime,
	} {
		if c.String() == "" {
			t.Errorf("class %d has no name", int(c))
		}
		back, ok := PriorityClassOfBase(c.Base())
		if !ok || back != c {
			t.Errorf("base priority %d maps to %v (ok=%v), want %v", c.Base(), back, ok, c)
		}
		if nice := c.Nice(); nice < -20 || nice > 19 {
			t.Errorf("%v nice = %d, out of the POSIX range", c, nice)
		}
	}
	if _, ok := PriorityClassOfBase(11); ok {
		t.Error("11 is not a priority class base priority")
	}
	// The classes keep the documented order: each step down is a higher nice.
	previous := -21
	for _, c := range []PriorityClass{PriorityHigh, PriorityAboveNormal, PriorityNormal, PriorityBelowNormal, PriorityIdle} {
		if c.Nice() <= previous {
			t.Errorf("%v nice = %d, want it above %d", c, c.Nice(), previous)
		}
		previous = c.Nice()
	}
}

func TestPriorityStepWithoutLevelsStepsOneNice(t *testing.T) {
	tests := []struct{ current, delta, want int }{
		{0, -1, -1},
		{0, 1, 1},
		{-20, -1, -20},
		{19, 1, 19},
		{5, 0, 5},
	}
	for _, tt := range tests {
		if got := PriorityStep(nil, tt.current, tt.delta); got != tt.want {
			t.Errorf("step(%d, %d) = %d, want %d", tt.current, tt.delta, got, tt.want)
		}
	}
}

func TestPriorityStepWalkLevels(t *testing.T) {
	levels := windowsPriorityLevels
	if got := PriorityStep(levels, 0, -1); got != -5 {
		t.Errorf("raise from normal = %d, want -5 (above normal)", got)
	}
	if got := PriorityStep(levels, 0, 1); got != 10 {
		t.Errorf("lower from normal = %d, want 10 (below normal)", got)
	}
	if got := PriorityStep(levels, -10, -1); got != -10 {
		t.Errorf("raise from the highest level = %d, want it unchanged", got)
	}
	if got := PriorityStep(levels, 19, 1); got != 19 {
		t.Errorf("lower from the lowest level = %d, want it unchanged", got)
	}
	// Nice 5 is between normal and below-normal, so it belongs to the
	// below-normal level and one step down is idle.
	if got := PriorityStep(levels, 5, 1); got != 19 {
		t.Errorf("step down from an in-between nice = %d, want 19", got)
	}
	if got := PriorityStep(levels, -7, -1); got != -10 {
		t.Errorf("step up from an in-between nice = %d, want -10", got)
	}
	for _, l := range levels {
		if got := PriorityStep(levels, l.Nice, 0); got != l.Nice {
			t.Errorf("zero step at %d = %d", l.Nice, got)
		}
	}
}

func TestPriorityLevelsAreOrderedAndSettable(t *testing.T) {
	previous := -21
	for _, l := range windowsPriorityLevels {
		if l.Nice <= previous {
			t.Fatalf("levels are not ordered by nice: %+v", windowsPriorityLevels)
		}
		if l.Name == "" {
			t.Errorf("level %d has no name", l.Nice)
		}
		if l.class == PriorityRealtime {
			t.Error("realtime must not be offered as a settable level")
		}
		previous = l.Nice
	}
	level, ok := PriorityLevelOf(windowsPriorityLevels, 3)
	if !ok || level.Nice != 10 {
		t.Errorf("nice 3 belongs to level %+v (ok=%v), want 10", level, ok)
	}
	if _, ok := PriorityLevelOf(nil, 0); ok {
		t.Error("a platform without levels reports none")
	}
}
