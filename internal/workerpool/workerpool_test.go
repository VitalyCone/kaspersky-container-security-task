package workerpool

import (
	// "errors"
	"log"
	"testing"
	"time"

	"github.com/VitalyCone/kaspersky-container-security-task/internal/domain/dto"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		queueSize int
		wantErr   error
	}{
		{
			name:      "Valid queue size",
			queueSize: 10,
			wantErr:   nil,
		},
		{
			name:      "Invalid queue size (zero)",
			queueSize: 0,
			wantErr:   ErrQueueSizeInvalid,
		},
		{
			name:      "Invalid queue size (negative)",
			queueSize: -1,
			wantErr:   ErrQueueSizeInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.queueSize, 0)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkerPool_AddTask(t *testing.T) {
	tests := []struct {
		name     string
		task     *Task
		poolSize int
		wantErr  error
	}{
		{
			name:     "Valid task",
			task:     &Task{Task: dto.Task{ID: "1"}},
			poolSize: 1,
			wantErr:  nil,
		},
		{
			name:     "Nil task",
			task:     nil,
			poolSize: 1,
			wantErr:  ErrInvalidTask,
		},
		{
			name:     "Queue full",
			task:     &Task{Task: dto.Task{ID: "1"}},
			poolSize: 1,
			wantErr:  ErrQueueFull,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wp, err := New(tt.poolSize, 0)
			assert.NoError(t, err)

			if i == 2 {
				wp.AddTask(tt.task)
			}
			err = wp.AddTask(tt.task)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkerPool_Stop(t *testing.T) {
	tests := []struct {
		name    string
		actions func(*WorkerPool) error
		wantErr error
	}{
		{
			name: "Normal stop",
			actions: func(wp *WorkerPool) error {
				wp.Start(1)
				return wp.Stop()
			},
			wantErr: nil,
		},
		{
			name: "Double stop",
			actions: func(wp *WorkerPool) error {
				wp.Start(1)
				_ = wp.Stop()
				return wp.Stop()
			},
			wantErr: ErrQueueClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wp, err := New(1, 0)
			assert.NoError(t, err)

			err = tt.actions(wp)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkerPool_TaskProcessing(t *testing.T) {
	wp, err := New(3, 0)
	assert.NoError(t, err)

	wp.Start(2)

	task := &Task{Task: dto.Task{ID: "1"}}
	err = wp.AddTask(task)
	assert.NoError(t, err)

	// Give some time for task processing
	time.Sleep(1000 * time.Millisecond)

	assert.Equal(t, StateDone, task.State)

	err = wp.Stop()
	assert.NoError(t, err)
}

func TestWorkerPool_ConcurrentTasks(t *testing.T) {
	wp, err := New(10, 0)
	assert.NoError(t, err)

	wp.Start(3)

	var tasks []*Task
	for i := 0; i < 5; i++ {
		task := &Task{Task: dto.Task{ID: "i"}}
		tasks = append(tasks, task)
		err = wp.AddTask(task)
		assert.NoError(t, err)
	}

	// Give some time for task processing
	time.Sleep(4000 * time.Millisecond)

	for _, task := range tasks {
		assert.Equal(t, StateDone, task.State)
	}

	err = wp.Stop()
	assert.NoError(t, err)
}

func TestWorkerPool_ClosedQueue(t *testing.T) {
	wp, err := New(1, 0)
	assert.NoError(t, err)

	wp.Start(1)
	err = wp.Stop()
	assert.NoError(t, err)

	task := &Task{Task: dto.Task{ID: "1"}}
	err = wp.AddTask(task)
	assert.ErrorIs(t, err, ErrQueueClosed)
}


func TestWorkerPool_TaskProcessWithRetry(t *testing.T) {
    wp, err := New(1, 0)
    assert.NoError(t, err)
    
    // Test with a task that will fail
	retries := 3
    task := &Task{Task: dto.Task{ID: "1", MaxRetries: retries}}
    err = wp.taskProcessWithRetry(task, 100)
	log.Println(err)
    assert.Error(t, err)
    assert.Equal(t, StateFailed, task.State)
    assert.Equal(t, retries, task.Retries) // Should have tried 3 times
}