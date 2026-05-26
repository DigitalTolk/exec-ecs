package cli

import "testing"

func TestMin(t *testing.T) {
	t.Parallel()
	if got := min(3, 7); got != 3 {
		t.Fatalf("min(3,7)=%d", got)
	}
	if got := min(7, 3); got != 3 {
		t.Fatalf("min(7,3)=%d", got)
	}
	if got := min(5, 5); got != 5 {
		t.Fatalf("min(5,5)=%d", got)
	}
}

func TestMax(t *testing.T) {
	t.Parallel()
	if got := max(3, 7); got != 7 {
		t.Fatalf("max(3,7)=%d", got)
	}
	if got := max(7, 3); got != 7 {
		t.Fatalf("max(7,3)=%d", got)
	}
	if got := max(5, 5); got != 5 {
		t.Fatalf("max(5,5)=%d", got)
	}
}
