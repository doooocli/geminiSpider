package main

import (
	"context"
	"sync"
)

type Scheduler struct {
	ctx context.Context
	sem chan struct{}
	wg  sync.WaitGroup
}

func NewScheduler(ctx context.Context, multi int) *Scheduler {
	scheduler := &Scheduler{
		ctx: ctx,
		sem: make(chan struct{}, multi),
	}
	go func() {
		<-ctx.Done()
		close(scheduler.sem)
	}()
	return scheduler
}

type handler func(ctx context.Context)

func (s *Scheduler) dispatch(fn handler) {
	select {
	case <-s.ctx.Done():
		return
	case s.sem <- struct{}{}: // 控制并发，会阻塞主进程
		s.wg.Add(1)
		go func() {
			defer func() {
				<-s.sem // 释放
				s.wg.Done()
			}()
			fn(s.ctx)
		}()
	}
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}
