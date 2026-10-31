package uutils

import "testing"

func TestFailedBeforeExecutionRecorded(t *testing.T) {
	status, reason, failure := resultStatus(true, true, &runRecord{})
	if status != "failed" || reason != nil || failure == "" {
		t.Fatalf("%q %+v %q", status, reason, failure)
	}
}
