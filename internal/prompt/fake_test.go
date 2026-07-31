package prompt

import "testing"

func TestScriptedReturnsQueuedAnswers(t *testing.T) {
	s := &Scripted{
		Inputs:       []string{"my-project"},
		Selects:      []int{0},
		MultiSelects: [][]int{{1}},
		Confirms:     []bool{true},
	}
	if v, _ := s.Input("name", "def"); v != "my-project" {
		t.Errorf("Input = %q", v)
	}
	if i, _ := s.Select("type", []Option{{Label: "init-loop"}}, 0); i != 0 {
		t.Errorf("Select = %d", i)
	}
	if idxs, _ := s.MultiSelect("ext", []Option{{Label: "date"}, {Label: "sqlite"}}); len(idxs) != 1 || idxs[0] != 1 {
		t.Errorf("MultiSelect = %v", idxs)
	}
	if b, _ := s.Confirm("minimal_tz", false); !b {
		t.Errorf("Confirm = %v", b)
	}
}
