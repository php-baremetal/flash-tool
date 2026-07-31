package prompt

// Scripted is a Prompter that returns queued answers in order — for tests. When a
// queue is exhausted it returns the prompt's default.
type Scripted struct {
	Inputs       []string
	Confirms     []bool
	Selects      []int
	MultiSelects [][]int

	inI, conI, selI, msI int
}

func (s *Scripted) Input(_, def string) (string, error) {
	if s.inI >= len(s.Inputs) {
		return def, nil
	}
	v := s.Inputs[s.inI]
	s.inI++
	return v, nil
}

func (s *Scripted) Confirm(_ string, def bool) (bool, error) {
	if s.conI >= len(s.Confirms) {
		return def, nil
	}
	v := s.Confirms[s.conI]
	s.conI++
	return v, nil
}

func (s *Scripted) Select(_ string, _ []Option, def int) (int, error) {
	if s.selI >= len(s.Selects) {
		return def, nil
	}
	v := s.Selects[s.selI]
	s.selI++
	return v, nil
}

func (s *Scripted) MultiSelect(_ string, _ []Option) ([]int, error) {
	if s.msI >= len(s.MultiSelects) {
		return nil, nil
	}
	v := s.MultiSelects[s.msI]
	s.msI++
	return v, nil
}
