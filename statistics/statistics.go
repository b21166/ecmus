package statistics

import (
	"fmt"
	"sync"
)

type statisticsData struct {
	dataMap map[string]int

	mutex sync.Mutex
}

var stats *statisticsData

func Init() {
	stats = &statisticsData{
		dataMap: make(map[string]int),
	}
}

func Set(key string, value int) {
	stats.mutex.Lock()
	defer stats.mutex.Unlock()

	stats.dataMap[key] = value
}

func Change(key string, value int) {
	stats.mutex.Lock()
	defer stats.mutex.Unlock()

	stats.dataMap[key] += value
}

func Display() string {
	stats.mutex.Lock()
	defer stats.mutex.Unlock()

	result := "Statistics results are:\n"
	for key, value := range stats.dataMap {
		result += fmt.Sprintf("Number of %s is %d\n", key, value)
	}

	return result
}
