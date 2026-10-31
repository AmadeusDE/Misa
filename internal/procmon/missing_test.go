package procmon

import "testing"

func TestCellRendersUnknownForMissingFields(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"known text", CellText("bash", true), "bash"},
		{"unknown text", CellText("bash", false), "?"},
		{"known left", CellTextLeft("alice", true, 8), "alice   "},
		{"unknown left", CellTextLeft("alice", false, 8), "?       "},
		{"known right", CellTextRight("1:02", true, 6), "  1:02"},
		{"unknown right", CellTextRight("1:02", false, 6), "     ?"},
		{"known int", CellInt(20, true, 3), " 20"},
		{"unknown int", CellInt(20, false, 3), "  ?"},
		{"known float", CellFloat(12.34, true, 5, 1), " 12.3"},
		{"unknown float", CellFloat(12.34, false, 5, 1), "    ?"},
		{"known bytes", CellBytes(2048, true, 7), "     2K"},
		{"unknown bytes", CellBytes(2048, false, 7), "      ?"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestProcessFieldFlags(t *testing.T) {
	for _, flags := range []struct {
		field string
		bit   ProcMissing
	}{
		{"user", MissingUser},
		{"state", MissingState},
		{"cpu time", MissingCPUTime},
		{"memory", MissingMemory},
		{"virt", MissingVirt},
		{"shared", MissingShared},
		{"mem pct", MissingMemPct},
		{"priority", MissingPriority},
	} {
		p := Process{Missing: flags.bit}
		for _, c := range []struct {
			field string
			has   bool
		}{
			{"user", p.HasUser()},
			{"state", p.HasState()},
			{"cpu time", p.HasCPUTime()},
			{"memory", p.HasMemory()},
			{"virt", p.HasVirt()},
			{"shared", p.HasShared()},
			{"mem pct", p.HasMemPct()},
			{"priority", p.HasPriority()},
		} {
			if want := c.field != flags.field; c.has != want {
				t.Errorf("Missing %s: %s available = %v, want %v", flags.field, c.field, c.has, want)
			}
		}
	}
	if !(Process{}).HasUser() || !(Process{}).HasPriority() {
		t.Error("a zero ProcMissing should report every field available")
	}
}

func TestSnapshotUptimeAndLoadStrings(t *testing.T) {
	snap := Snapshot{SystemAvailable: true, Uptime: 3661, LoadAvg: [3]float64{0.5, 0.25, 0}}
	if got, want := snap.UptimeStr(), "1:01"; got != want {
		t.Errorf("uptime = %q, want %q", got, want)
	}
	if got, want := snap.LoadStr(), "0.50 0.25 0.00"; got != want {
		t.Errorf("load = %q, want %q", got, want)
	}

	snap.SystemAvailable = false
	if snap.UptimeStr() != unavailable || snap.LoadStr() != unavailable {
		t.Errorf("unavailable platform rendered uptime %q load %q", snap.UptimeStr(), snap.LoadStr())
	}
}

func TestStatesKnown(t *testing.T) {
	if StatesKnown(nil) {
		t.Error("an empty process list has no states")
	}
	if !StatesKnown([]Process{{PID: 1, State: 'S'}}) {
		t.Error("a process with a state should report counts as known")
	}
	if StatesKnown([]Process{{PID: 1, State: 'S'}, {PID: 2, Missing: MissingState}}) {
		t.Error("one process without a state makes the counts unknown")
	}
}
