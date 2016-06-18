package rotate

import "testing"

func TestGetLogName(t *testing.T) {
	name := getLogName()
	if len(name) != 26 {
		t.Fatalf("unexpected %q", name)
	}
}
