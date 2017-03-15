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
	"fmt"
	"time"

	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/util/workqueue"

	"github.com/golang/glog"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

// TaskQueue manages a work queue through an independent worker that
// invokes the given sync function for every work item inserted.
type TaskQueue struct {
	// queue is the work queue the worker polls
	queue *workqueue.Type
	// sync is called for each item in the queue
	sync func(interface{}) error
	// workerDone is closed when the worker exits
	workerDone chan struct{}
	// keyFn function (default if one is not supplied to New)
	keyFn func(obj interface{}) (interface{}, error)
}

// Run ...
func (t *TaskQueue) Run(period time.Duration, stopCh <-chan struct{}) {
	wait.Until(t.worker, period, stopCh)
}

// Enqueue enqueues ns/name of the given api object in the task queue.
func (t *TaskQueue) Enqueue(obj interface{}) {
	if t.IsShuttingDown() {
		glog.Errorf("queue has been shutdown, failed to enqueue: %v", obj)
		return
	}

	key, err := keyFunc(obj)
	if err != nil {
		glog.V(3).Infof("couldn't get key for object %+v: %v", obj, err)
		return
	}

	glog.V(3).Infof("queuing: %s", key)
	glog.V(4).Infof("key referenced obj: %v", obj)
	t.queue.Add(key)
}

// Requeue - enqueues ns/name of the given api object in the task queue.
func (t *TaskQueue) Requeue(key string, err error) {
	glog.Warningf("requeuing %v, err %v", key, err)
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

		keyValue, ok := key.(string)
		if !ok {
			glog.Warningf("invalid key: %v", key)
		}

		glog.V(3).Infof("syncing: %s", keyValue)
		if err := t.sync(keyValue); err != nil {
			t.Requeue(keyValue, err)
		}
		t.queue.Done(key)
	}
}

// IsShuttingDown returns if the method Shutdown was invoked
func (t *TaskQueue) IsShuttingDown() bool {
	return t.queue.ShuttingDown()
}

// Shutdown shuts down the work queue and waits for the worker to ACK
func (t *TaskQueue) Shutdown() {
	t.queue.ShutDown()
	<-t.workerDone
}

// default keyFn if a user func isn't supplied
func (t *TaskQueue) defaultKeyFunc(obj interface{}) (interface{}, error) {
	key, err := keyFunc(obj)
	if err != nil {
		return "", fmt.Errorf("could not get key for object %+v: %v", obj, err)
	}

	return key, nil
}

// NewTaskQueue creates a new task queue with the given sync function.
// The sync function is called for every element inserted into the queue.
func NewTaskQueue(syncFn func(interface{}) error) *TaskQueue {
	return NewTaskQueueKeyFn(syncFn, nil)
}

// NewTaskQueueKeyFn creates a new task queue with the given sync function and
// API Object Key generator function.
// The user's sync function is called for every element inserted into the queue.
func NewTaskQueueKeyFn(syncFn func(interface{}) error, keyFn func(interface{}) (interface{}, error)) *TaskQueue {
	taskQueue := &TaskQueue{
		queue:      workqueue.New(),
		sync:       syncFn,
		workerDone: make(chan struct{}),
		keyFn:      keyFn,
	}

	if taskQueue.keyFn == nil {
		taskQueue.keyFn = taskQueue.defaultKeyFunc
	}

	return taskQueue
}
