package jobs

import (
	"sync"
	"sync/atomic"
)

type jobRunner struct {
	running atomic.Bool
	taskMu  sync.RWMutex
	task    func()
}

var jobRunners = map[string]*jobRunner{}

func configureJobRunner(name string, task func()) *jobRunner {
	runner := jobRunners[name]
	if runner == nil {
		runner = &jobRunner{}
		jobRunners[name] = runner
	}
	runner.setTask(task)
	return runner
}

func (r *jobRunner) setTask(task func()) {
	r.taskMu.Lock()
	r.task = task
	r.taskMu.Unlock()
}

func (r *jobRunner) runScheduled() {
	if !r.running.CompareAndSwap(false, true) {
		return
	}
	r.execute()
}

func (r *jobRunner) trigger() bool {
	if !r.running.CompareAndSwap(false, true) {
		return false
	}
	go r.execute()
	return true
}

func (r *jobRunner) execute() {
	defer r.running.Store(false)

	r.taskMu.RLock()
	task := r.task
	r.taskMu.RUnlock()
	if task != nil {
		task()
	}
}
