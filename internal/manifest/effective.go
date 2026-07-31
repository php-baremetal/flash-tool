package manifest

import "sort"

type Selection struct {
	Enabled  map[string]bool
	Settings map[string]map[string]bool
}

type Effective struct {
	Flags    []string
	PulledIn []string
	Fetches  []string
}

type UnknownRequireError struct{ Key string }

func (e *UnknownRequireError) Error() string {
	return "unknown extension required: " + e.Key
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func (m *Manifest) byKey() map[string]*Extension {
	idx := map[string]*Extension{}
	for i := range m.Extensions {
		idx[m.Extensions[i].Key] = &m.Extensions[i]
	}
	return idx
}

// OfferedModes are the storage/project types that are both available in the manifest
// and supported by the board.
func (m *Manifest) OfferedModes(board *Board) (storage, project []Mode) {
	for _, s := range m.StorageTypes {
		if s.Available && contains(board.StorageTypes, s.Key) {
			storage = append(storage, s)
		}
	}
	for _, p := range m.ProjectTypes {
		if p.Available && contains(board.ProjectTypes, p.Key) {
			project = append(project, p)
		}
	}
	return
}

// MandatoryFor lists extensions whose required_for includes the project type.
func (m *Manifest) MandatoryFor(projectType string) []Extension {
	var out []Extension
	for _, e := range m.Extensions {
		if contains(e.RequiredFor, projectType) {
			out = append(out, e)
		}
	}
	return out
}

// OptionalFor lists the toggleable extensions (they carry a flag and aren't mandatory
// for the project type).
func (m *Manifest) OptionalFor(projectType string) []Extension {
	var out []Extension
	for _, e := range m.Extensions {
		if e.Flag != "" && !contains(e.RequiredFor, projectType) {
			out = append(out, e)
		}
	}
	return out
}

// Effective resolves the type's mandatory extensions plus the enabled optional ones
// (with transitive requires) into the sorted -D flag list, the pulled-in dependency
// keys, and the fetch scripts to run. It errors if an enabled extension requires an
// unknown key.
func (m *Manifest) Effective(projectType string, sel Selection) (*Effective, error) {
	idx := m.byKey()
	active := map[string]bool{}
	var pulledIn []string

	for _, e := range m.MandatoryFor(projectType) {
		active[e.Key] = true
	}

	var enable func(key string, viaDep bool) error
	enable = func(key string, viaDep bool) error {
		if active[key] {
			return nil
		}
		e, ok := idx[key]
		if !ok {
			return &UnknownRequireError{Key: key}
		}
		active[key] = true
		if viaDep {
			pulledIn = append(pulledIn, key)
		}
		for _, dep := range e.Requires {
			if err := enable(dep, true); err != nil {
				return err
			}
		}
		return nil
	}
	for key, on := range sel.Enabled {
		if on {
			if err := enable(key, false); err != nil {
				return nil, err
			}
		}
	}

	var flags, fetches []string
	for key := range active {
		e := idx[key]
		if e.Flag != "" {
			flags = append(flags, e.Flag)
		}
		if e.Fetch != "" {
			fetches = append(fetches, e.Fetch)
		}
		for _, s := range e.Settings {
			if sel.Settings[key][s.Key] {
				flags = append(flags, s.Flag)
				if s.Fetch != "" {
					fetches = append(fetches, s.Fetch)
				}
			}
		}
	}
	sort.Strings(flags)
	sort.Strings(fetches)
	sort.Strings(pulledIn)
	return &Effective{Flags: flags, PulledIn: pulledIn, Fetches: fetches}, nil
}
