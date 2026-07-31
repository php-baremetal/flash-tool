package manifest

import (
	"reflect"
	"testing"
)

func testManifest() *Manifest {
	return &Manifest{
		StorageTypes: []Mode{{Key: "microsd", Available: true}, {Key: "embedded", Available: false}},
		ProjectTypes: []Mode{{Key: "init-loop", Available: true}, {Key: "web-server", Available: false}},
		Extensions: []Extension{
			{Key: "gpio", RequiredFor: []string{"init-loop"}},
			{Key: "pdo", Flag: "PHP_EXT_PDO=ON"},
			{Key: "sqlite", Flag: "PHP_EXT_SQLITE=ON", Requires: []string{"pdo"}, Fetch: "scripts/fetch-sqlite.sh"},
			{Key: "date", Flag: "PHP_EXT_DATE=ON", Settings: []Setting{
				{Key: "minimal_tz", Flag: "PHP_EXT_DATE_MINIMAL_TZ=ON"},
			}},
		},
	}
}

func TestOfferedModes(t *testing.T) {
	m := testManifest()
	board := &Board{StorageTypes: []string{"microsd", "embedded"}, ProjectTypes: []string{"init-loop", "event-driven"}}
	storage, project := m.OfferedModes(board)
	if len(storage) != 1 || storage[0].Key != "microsd" {
		t.Errorf("storage offered = %+v", storage)
	}
	if len(project) != 1 || project[0].Key != "init-loop" {
		t.Errorf("project offered = %+v", project)
	}
}

func TestEffectivePullsInRequires(t *testing.T) {
	m := testManifest()
	sel := Selection{
		Enabled:  map[string]bool{"sqlite": true, "date": true},
		Settings: map[string]map[string]bool{"date": {"minimal_tz": true}},
	}
	eff, err := m.Effective("init-loop", sel)
	if err != nil {
		t.Fatal(err)
	}
	wantFlags := []string{"PHP_EXT_DATE=ON", "PHP_EXT_DATE_MINIMAL_TZ=ON", "PHP_EXT_PDO=ON", "PHP_EXT_SQLITE=ON"}
	if !reflect.DeepEqual(eff.Flags, wantFlags) {
		t.Errorf("flags = %v, want %v", eff.Flags, wantFlags)
	}
	if !reflect.DeepEqual(eff.PulledIn, []string{"pdo"}) {
		t.Errorf("pulledIn = %v", eff.PulledIn)
	}
	if !reflect.DeepEqual(eff.Fetches, []string{"scripts/fetch-sqlite.sh"}) {
		t.Errorf("fetches = %v", eff.Fetches)
	}
}

func TestEffectiveUnknownRequire(t *testing.T) {
	m := &Manifest{Extensions: []Extension{{Key: "x", Flag: "PHP_EXT_X=ON", Requires: []string{"ghost"}}}}
	_, err := m.Effective("init-loop", Selection{Enabled: map[string]bool{"x": true}})
	if err == nil {
		t.Fatal("want error for unknown require 'ghost'")
	}
}
