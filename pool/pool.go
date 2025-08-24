package pool

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var timeNow = time.Now

type (
	object            = comparable
	Adaptor[T object] interface {
		New(ctx context.Context, key string) (T, error)
		Check(T) error
		Close(T) error
	}
)

func New[T object](expire time.Duration, adaptor Adaptor[T]) Pool[T] {
	return &pool[T]{
		expire:   expire,
		adaptor:  adaptor,
		items:    make(map[string]T),
		lastUsed: make(map[string]time.Time),
		closed:   make(chan struct{}),
	}
}

type Pool[T object] interface {
	Get(ctx context.Context, key string) (T, error)
	Close()
}

type pool[T object] struct {
	_ [0]func()

	expire  time.Duration
	adaptor Adaptor[T]

	rw sync.RWMutex

	once sync.Once
	sf   singleflight.Group

	empty T

	items    map[string]T
	lastUsed map[string]time.Time

	closeOnce sync.Once
	closed    chan struct{}
}

func (p *pool[T]) Get(ctx context.Context, key string) (T, error) {
	p.once.Do(func() { go p.gcStart() })

	item := p.getFast(key)
	if item == p.empty {
		var err error
		item, err = p.getSlow(ctx, key)
		if err != nil {
			return p.empty, err
		}
	}

	if err := p.adaptor.Check(item); err != nil {
		p.rw.Lock()
		defer p.rw.Unlock()
		p.delete(key)
		return p.empty, err
	}

	return item, nil
}

func (p *pool[T]) getFast(key string) T {
	p.rw.RLock()
	defer p.rw.RUnlock()

	return p.get(key)
}

func (p *pool[T]) getSlow(ctx context.Context, key string) (T, error) {
	if _, err, _ := p.sf.Do(key, func() (interface{}, error) {
		get := p.getFast(key)
		if get != p.empty {
			// singleflight + rlock check
			return nil, nil
		}

		item, err := p.adaptor.New(ctx, key)
		if err != nil {
			return nil, err
		}

		p.rw.Lock()
		defer p.rw.Unlock()
		p.items[key] = item
		return nil, nil
	}); err != nil {
		return p.empty, err
	}

	return p.getFast(key), nil
}

func (p *pool[T]) get(key string) T {
	item := p.items[key]
	if item == p.empty {
		return p.empty
	}

	p.lastUsed[key] = timeNow()
	return item
}

func (p *pool[T]) Close() {
	p.closeOnce.Do(func() { close(p.closed) })
}

func (p *pool[T]) gcStart() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		var closed bool
		select {
		case <-p.closed:
			closed = true
		case <-ticker.C:
		}

		p.gc(closed)
		if closed {
			slog.Info("pool closed")
			return
		}
	}
}

func (p *pool[T]) gc(all bool) {
	p.rw.Lock()
	defer p.rw.Unlock()

	now := timeNow()
	expire := now.Add(-p.expire)
	var expired []string
	for key, ti := range p.lastUsed {
		if ti.Before(expire) || all {
			expired = append(expired, key)
		}
	}

	for _, key := range expired {
		p.delete(key)
	}
}

func (p *pool[T]) delete(key string) {
	c := p.items[key]
	delete(p.items, key)
	delete(p.lastUsed, key)

	if c == p.empty {
		return
	}

	if err := p.adaptor.Close(c); err != nil {
		slog.Warn("failed to close", slog.String("key", key), slog.String("error", err.Error()))
	}
}
