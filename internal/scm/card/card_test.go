package card

import "testing"

func TestEmptyProperties(t *testing.T) {
	t.Parallel()
	p := EmptyProperties()
	if len(p) != 0 {
		t.Fatal("expected empty map")
	}
}
