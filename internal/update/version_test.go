package update

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		left  string
		right string
		want  int
	}{
		{left: "v0.79", right: "0.78", want: 1},
		{left: "0.78", right: "v0.78", want: 0},
		{left: "1.2.0", right: "1.10.0", want: -1},
		{left: "1.10.1", right: "1.10.0", want: 1},
	}
	for _, tc := range cases {
		if got := compareVersions(tc.left, tc.right); got != tc.want {
			t.Fatalf("compareVersions(%q, %q) = %d, want %d", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestDisplayVersion(t *testing.T) {
	if got := displayVersion("0.78"); got != "v0.78" {
		t.Fatalf("expected v0.78, got %q", got)
	}
	if got := displayVersion("v0.78"); got != "v0.78" {
		t.Fatalf("expected v0.78, got %q", got)
	}
}
