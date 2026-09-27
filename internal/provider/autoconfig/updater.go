package autoconfig

import (
	"reflect"
	"sync"
	"time"
)

type Closer func()
type Parser[T any] func(base T, raw map[string]any) (T, error)
type Equal[T any] func(a, b T) bool
type Recreate[T any] func(oldCfg, newCfg T) bool
type Build[T any, C any] func(cfg T) (C, error)
type Close[C any] func(client C)

type State[T any, C any] struct {
	mu           *sync.RWMutex
	config       T
	client       C
	pendingClose []C
	closeFn      Close[C]
	draining     bool
}

func NewState[T any, C any](mu *sync.RWMutex, cfg T, client C) *State[T, C] {
	return &State[T, C]{mu: mu, config: cfg, client: client}
}

func (s *State[T, C]) Config() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *State[T, C]) Client() C {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

type Options[T any, C any] struct {
	Parse     Parser[T]
	Equal     Equal[T]
	Recreate  Recreate[T]
	Build     Build[T, C]
	Close     Close[C]
	OnChanged func(oldCfg, newCfg T, recreated bool)
	OnError   func(err error)
}

func Apply[T any, C any](s *State[T, C], raw map[string]any, opts Options[T, C]) (changed bool, recreated bool, err error) {
	if opts.Parse == nil {
		panic("autoconfig: nil Parse")
	}

	if s.closeFn == nil && opts.Close != nil {
		s.closeFn = opts.Close
	}

	base := s.Config()
	newCfg, err := opts.Parse(base, raw)
	if err != nil {
		if opts.OnError != nil {
			opts.OnError(err)
		}
		return false, false, err
	}

	equal := opts.Equal
	if equal == nil {
		equal = func(a, b T) bool { return reflect.DeepEqual(a, b) }
	}
	if equal(base, newCfg) {
		return false, false, nil
	}

	recreate := opts.Recreate
	if recreate == nil {
		recreate = func(oldCfg, newCfg T) bool { return true }
	}
	shouldRecreate := recreate(base, newCfg)

	var newClient C
	if shouldRecreate && opts.Build != nil {
		newClient, err = opts.Build(newCfg)
		if err != nil {
			if opts.OnError != nil {
				opts.OnError(err)
			}
			return false, false, err
		}
	}

	s.mu.Lock()
	currentCfg := s.config
	if equal(currentCfg, newCfg) {
		s.mu.Unlock()
		if shouldRecreate && opts.Close != nil {
			opts.Close(newClient)
		}
		return false, false, nil
	}
	shouldRecreate = recreate(currentCfg, newCfg)
	oldClient := s.client
	s.config = newCfg
	if shouldRecreate && opts.Build != nil {
		s.client = newClient
	}
	if shouldRecreate && opts.Close != nil {
		s.pendingClose = append(s.pendingClose, oldClient)
		if !s.draining {
			s.draining = true
			go s.drainPendingClose(5 * time.Second)
		}
	}
	s.mu.Unlock()

	clientRecreated := shouldRecreate && opts.Build != nil
	if opts.OnChanged != nil {
		opts.OnChanged(currentCfg, newCfg, clientRecreated)
	}
	return true, clientRecreated, nil
}

func (s *State[T, C]) drainPendingClose(grace time.Duration) {
	time.Sleep(grace)
	s.mu.Lock()
	toClose := s.pendingClose
	s.pendingClose = nil
	s.draining = false
	s.mu.Unlock()
	for _, c := range toClose {
		if s.closeFn != nil {
			s.closeFn(c)
		}
	}
}
