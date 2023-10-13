package scheduler

type healthCheckSample struct {
	expectationsLength int
	lastExpectationId  uint32
}

func newHealthCheckSample(scheduler *Scheduler) *healthCheckSample {

	var lastExpectationId uint32
	if len(scheduler.expectations) > 0 {
		lastExpectationId = scheduler.expectations[len(scheduler.expectations)-1].id
	}

	newSample := &healthCheckSample{
		expectationsLength: len(scheduler.expectations),
		lastExpectationId:  lastExpectationId,
	}

	return newSample
}

func (h *healthCheckSample) isTheSame(o *healthCheckSample) bool {
	if o == nil {
		return false
	}

	if o.expectationsLength != h.expectationsLength {
		return false
	}

	if o.lastExpectationId != h.lastExpectationId {
		return false
	}

	return true
}
