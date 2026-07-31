package config

import "testing"

func TestResolvePrecedence(t *testing.T) {
	cases := []struct {
		name string
		in   ResolveInputs
		want Source
	}{
		{"flag wins", ResolveInputs{Flag: "/f", Config: "/c", Env: "/e", Default: "/d"}, Source{"/f", "flag"}},
		{"config next", ResolveInputs{Config: "/c", Env: "/e", Default: "/d"}, Source{"/c", "config"}},
		{"env next", ResolveInputs{Env: "/e", Default: "/d"}, Source{"/e", "env"}},
		{"default last", ResolveInputs{Default: "/d"}, Source{"/d", "default"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Resolve(tc.in)
			if got != tc.want {
				t.Errorf("Resolve(%+v) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}
