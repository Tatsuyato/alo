package main

import (
	"fmt"
	"sync"
	"time"
)

type JobRunner func(*Job)

type Queue struct {
	jobs    map[string]*Job
	runners map[string]JobRunner
	q       chan string
	stopCh  chan struct{}
	mu      sync.RWMutex
	wg      sync.WaitGroup
}

func NewQueue(workers int) *Queue {
	q := &Queue{
		jobs:    map[string]*Job{},
		runners: map[string]JobRunner{},
		q:       make(chan string, 256),
		stopCh:  make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker()
	}
	return q
}

func (q *Queue) Stop() {
	close(q.stopCh)
	q.wg.Wait()
}

func (q *Queue) RegisterRunner(jobType string, runner JobRunner) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.runners[jobType] = runner
}

func (q *Queue) Enqueue(jobType string, payload any, maxRetries int) *Job {
	id := fmt.Sprintf("job-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	job := &Job{
		ID:         id,
		Type:       jobType,
		Status:     JobQueued,
		Attempts:   0,
		MaxRetries: maxRetries,
		CreatedAt:  now,
		UpdatedAt:  now,
		Payload:    payload,
	}
	q.mu.Lock()
	q.jobs[id] = job
	q.mu.Unlock()
	q.q <- id
	return job
}

func (q *Queue) Get(id string) (*Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	job, ok := q.jobs[id]
	if !ok {
		return nil, false
	}
	copied := *job
	return &copied, true
}

func (q *Queue) update(id string, fn func(*Job)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if job, ok := q.jobs[id]; ok {
		fn(job)
		job.UpdatedAt = time.Now().UTC()
	}
}

func (q *Queue) worker() {
	defer q.wg.Done()
	for {
		select {
		case <-q.stopCh:
			return
		case id := <-q.q:
			job, ok := q.Get(id)
			if !ok {
				continue
			}
			q.mu.RLock()
			runner := q.runners[job.Type]
			q.mu.RUnlock()
			if runner == nil {
				q.update(id, func(j *Job) {
					j.Status = JobFailed
					j.Error = "missing runner"
				})
				continue
			}
			q.update(id, func(j *Job) {
				j.Status = JobRunning
				j.Attempts++
			})
			runner(job)
		}
	}
}
