package testing_tool

type PodOnNodeOccurrencesSorter struct {
	objects []*NodeDesc
}

func (ponos *PodOnNodeOccurrencesSorter) Len() int {
	return len(ponos.objects)
}

func (ponos *PodOnNodeOccurrencesSorter) Swap(i, j int) {
	ponos.objects[i], ponos.objects[j] = ponos.objects[j], ponos.objects[i]
}

func (ponos *PodOnNodeOccurrencesSorter) Less(i, j int) bool {
	if ponos.objects[i].Cpu != ponos.objects[j].Cpu {
		return ponos.objects[i].Cpu < ponos.objects[j].Cpu
	}

	if ponos.objects[i].Memory != ponos.objects[j].Memory {
		if ponos.objects[i].Memory != ponos.objects[j].Memory {
			return ponos.objects[i].Memory < ponos.objects[j].Memory
		}
	}

	// They are equal.
	return false
}
