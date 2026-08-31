package jobs

import (
	"testing"
	"time"
)

func TestJobRunnerRejectsManualTriggerWhileScheduledRunIsActive(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})

	runner := &jobRunner{}
	runner.setTask(func() {
		close(entered)
		<-release
	})

	go func() {
		runner.runScheduled()
		close(finished)
	}()
	<-entered

	if runner.trigger() {
		t.Fatal("manual trigger was accepted while scheduled run was active")
	}

	close(release)
	<-finished
}

func TestConfigureJobRunnerKeepsGateAcrossTaskReplacement(t *testing.T) {
	previous := jobRunners
	jobRunners = map[string]*jobRunner{}
	t.Cleanup(func() { jobRunners = previous })

	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})

	first := configureJobRunner("AutoDownload Job", func() {
		close(entered)
		<-release
	})
	go func() {
		first.runScheduled()
		close(finished)
	}()
	<-entered

	secondRan := make(chan struct{})
	second := configureJobRunner("AutoDownload Job", func() { close(secondRan) })
	if first != second {
		t.Fatal("task replacement created a new runner")
	}
	if second.trigger() {
		t.Fatal("replacement runner accepted a trigger while previous task was active")
	}

	close(release)
	<-finished
	if !second.trigger() {
		t.Fatal("replacement runner remained busy after previous task finished")
	}
	select {
	case <-secondRan:
	case <-time.After(5 * time.Second):
		t.Fatal("replacement task did not run")
	}
}
