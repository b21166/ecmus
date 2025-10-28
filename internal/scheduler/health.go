package scheduler

type healthCheckSample struct {
	expectationsLength int
	lastExpectation    *expectation
}

func newHealthCheckSample(scheduler *Scheduler) *healthCheckSample {

	var lastExpectation *expectation
	if len(scheduler.expectations) > 0 {
		lastExpectation = scheduler.expectations[len(scheduler.expectations)-1]
	} else {
		lastExpectation = nil
	}

	newSample := &healthCheckSample{
		expectationsLength: len(scheduler.expectations),
		lastExpectation:    lastExpectation,
	}

	return newSample
}

func (h *healthCheckSample) isStuck(o *healthCheckSample) bool {
	if o == nil {
		return false
	}

	if o.expectationsLength != h.expectationsLength || h.expectationsLength == 0 {
		return false
	}

	if o.lastExpectation.id != h.lastExpectation.id {
		return false
	}

	// if o.lastExpectation.tp == PLACING {
	// 	return false
	// }

	return true
}
