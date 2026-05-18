package worker_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"support-ticket.com/internal/worker"
)

func TestWorkerRun(t *testing.T) {
	// Example processing func
	processFunc := func(val int) worker.JobResult[string] {
		// simulate some work
		time.Sleep(10 * time.Millisecond)
		if val < 0 {
			return worker.JobResult[string]{Err: errors.New("negative value")}
		}
		return worker.JobResult[string]{Value: strconv.Itoa(val)}
	}

	tests := []struct {
		name        string
		items       []int
		jobFunc     func(int) worker.JobResult[string]
		expectedLen int
		verify      func(*testing.T, []worker.JobResult[string])
	}{
		{
			name:        "Success_ProcessMultipleItems",
			items:       []int{1, 2, 3, 4, 5},
			jobFunc:     processFunc,
			expectedLen: 5,
			verify: func(t *testing.T, results []worker.JobResult[string]) {
				// Validate we have the values somewhere (results order is not guaranteed)
				valuesMap := make(map[string]bool)
				for _, r := range results {
					assert.NoError(t, r.Err)
					valuesMap[r.Value] = true
				}
				assert.True(t, valuesMap["1"])
				assert.True(t, valuesMap["5"])
			},
		},
		{
			name:        "EmptyItems",
			items:       []int{},
			jobFunc:     processFunc,
			expectedLen: 0,
			verify: func(t *testing.T, results []worker.JobResult[string]) {
				assert.Empty(t, results)
			},
		},
		{
			name:        "WithError",
			items:       []int{1, -1, 2},
			jobFunc:     processFunc,
			expectedLen: 3,
			verify: func(t *testing.T, results []worker.JobResult[string]) {
				errCount := 0
				valCount := 0
				for _, r := range results {
					if r.Err != nil {
						errCount++
						assert.EqualError(t, r.Err, "negative value")
					} else {
						valCount++
					}
				}
				assert.Equal(t, 1, errCount)
				assert.Equal(t, 2, valCount)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := worker.Run(tt.items, tt.jobFunc)
			assert.Len(t, results, tt.expectedLen)
			if tt.verify != nil {
				tt.verify(t, results)
			}
		})
	}
}
