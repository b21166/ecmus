package alg

type Sorter[Obj any] struct {
	objects []*Obj
	by      func(*Obj) float64
}

func (s *Sorter[Obj]) Len() int {
	return len(s.objects)
}

func (s *Sorter[Obj]) Swap(i, j int) {
	s.objects[i], s.objects[j] = s.objects[j], s.objects[i]
}

func (s *Sorter[obj]) Less(i, j int) bool {
	return s.by(s.objects[i]) < s.by(s.objects[j])
}
