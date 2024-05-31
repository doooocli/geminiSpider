package main

import "sync"

type SafeQueue[T any] struct {
	queue []T
	lock  sync.Mutex
}

func NewSafeQueue[T any]() *SafeQueue[T] {
	return &SafeQueue[T]{
		queue: make([]T, 0),
	}
}

func (q *SafeQueue[T]) IsEmpty() bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	return len(q.queue) == 0
}

func (q *SafeQueue[T]) Push(item T) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.queue = append(q.queue, item)
}

func (q *SafeQueue[T]) Pop() (T, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	var zero T
	if len(q.queue) == 0 {
		return zero, false
	}

	item := q.queue[0]
	q.queue = q.queue[1:]
	return item, true
}
