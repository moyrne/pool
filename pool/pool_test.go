package pool

import (
	"context"
	"strconv"
	"testing"
	"time"
)

type Item struct {
	Name string
}

var _ Adaptor[*Item] = (*ItemAdaptor)(nil)

type ItemAdaptor struct{}

func (i *ItemAdaptor) New(ctx context.Context, key string) (*Item, error) { return &Item{key}, nil }
func (i *ItemAdaptor) Check(t *Item) error                                { return nil }
func (i *ItemAdaptor) Close(t *Item) error                                { return nil }

func TestMemPool(t *testing.T) {
	p := New(time.Second*10, &ItemAdaptor{})
	for i := 0; i < 10; i++ {
		val, err := p.Get(context.Background(), strconv.Itoa(i))
		if err != nil {
			t.Errorf("Get(%d) failed: %v", i, err)
		}
		if val.Name != strconv.Itoa(i) {
			t.Errorf("Get(%d) failed: expected %s, got %s", i, val.Name, strconv.Itoa(i))
		}
	}

	pp := p.(*pool[*Item])
	func() {
		pp.rw.Lock()
		defer pp.rw.Unlock()

		if len(pp.items) != 10 {
			t.Errorf("len(pp.items) = %d, want %d", len(pp.items), 10)
		}
	}()

	timeNow = func() time.Time { return time.Now().Add(time.Second * 15) }

	pp.gc(false)
	func() {
		pp.rw.Lock()
		defer pp.rw.Unlock()

		if len(pp.items) != 0 {
			t.Errorf("len(pp.items) = %d, want %d", len(pp.items), 0)
		}
	}()
}
