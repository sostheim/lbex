/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"time"

	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/util/workqueue"

	"github.com/golang/glog"
)

var keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc

// TaskQueue manages a work queue through an independent worker that
// invokes the given sync function for every work item inserted.
type TaskQueue struct {
	// queue is the work queue the worker polls
	queue *workqueue.Type
	// sync is called for each item in the queue
	sync func(string)
	// workerDone is closed when the worker exits
	workerDone chan struct{}
}

func (t *TaskQueue) run(period time.Duration, stopCh <-chan struct{}) {
	wait.Until(t.worker, period, stopCh)
}

// Enqueue enqueues ns/name of the given api object in the task queue.
func (t *TaskQueue) Enqueue(obj interface{}) {
	key, err := keyFunc(obj)
	if err != nil {
		glog.V(3).Infof("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	t.queue.Add(key)
}

// Requeue - enqueues ns/name of the given api object in the task queue.
func (t *TaskQueue) Requeue(key string, err error) {
	glog.Errorf("Requeuing %v, err %v", key, err)
	t.queue.Add(key)
}

// worker processes work in the queue through sync.
func (t *TaskQueue) worker() {
	for {
		key, quit := t.queue.Get()
		if quit {
			close(t.workerDone)
			return
		}
		glog.V(3).Infof("Syncing %v", key)
		t.sync(key.(string))
		t.queue.Done(key)
	}
}

// Shutdown shuts down the work queue and waits for the worker to ACK
func (t *TaskQueue) Shutdown() {
	t.queue.ShutDown()
	<-t.workerDone
}

// NewTaskQueue creates a new task queue with the given sync function.
// The sync function is called for every element inserted into the queue.
func NewTaskQueue(syncFn func(string)) *TaskQueue {
	return &TaskQueue{
		queue:      workqueue.New(),
		sync:       syncFn,
		workerDone: make(chan struct{}),
	}
}
