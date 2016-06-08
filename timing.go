package timing

import (
	"container/heap"
	"sync"
	"time"
)

const (
	kTimingBulkSize            int = 1000
	kTimingBulkItemDefaultSize int = 100
)

type item struct {
	circle int
	ch     chan struct{}
}

type bulk struct {
	capacity int
	size     int
	index    int
	bits     map[int]*item
	items    []*item
}

func newBulk(idx int) *bulk {
	return &bulk{
		capacity: kTimingBulkItemDefaultSize,
		size:     0,
		index:    idx,
		bits:     make(map[int]*item),
		items:    make([]*item, kTimingBulkItemDefaultSize),
	}
}

func (b bulk) Len() int {
	return b.size
}

func (b bulk) Less(i, j int) bool {
	return b.items[i].circle < b.items[j].circle
}

func (b bulk) Swap(i, j int) {
	b.items[i], b.items[j] = b.items[j], b.items[i]
}

func (b *bulk) Push(x interface{}) {
	v, ok := x.(item)
	if !ok {
		return
	}

	_, ok = b.bits[v.circle]
	if ok {
		return
	}

	if b.size >= b.capacity {
		b.items = append(b.items, b.items...)
		b.capacity *= 2
	}

	v.ch = make(chan struct{})
	b.items[b.size] = &v
	b.bits[v.circle] = b.items[b.size]
	b.size += 1
}

func (b *bulk) Pop() interface{} {
	if b.size <= 0 {
		return nil
	}

	b.size -= 1

	x := b.items[b.size]

	if b.size < b.capacity/2 && b.capacity > kTimingBulkItemDefaultSize {
		b.items = b.items[:b.capacity/2]
		b.capacity /= 2
	}

	delete(b.bits, x.circle)

	close(x.ch)

	return x
}

func (b *bulk) after(circle int) <-chan struct{} {
	i := item{
		circle: circle,
	}

	heap.Push(b, i)

	v, ok := b.bits[circle]

	if !ok {
		panic("bulk After error")
	}

	return v.ch
}

func (b *bulk) onTicker(circle int) {
	if b.size <= 0 {
		return
	}
	i := b.items[0]
	if i.circle <= circle {
		heap.Pop(b)
	}
}

type TimingWheel struct {
	sync.Mutex

	interval time.Duration

	circle   int
	position int

	ticker *time.Ticker

	quit chan struct{}

	bulks [kTimingBulkSize]*bulk
}

func NewTimingWheel() *TimingWheel {
	w := &TimingWheel{
		interval: time.Millisecond,
		circle:   0,
		position: 0,
		quit:     make(chan struct{}),
	}

	for i := 0; i < kTimingBulkSize; i++ {
		w.bulks[i] = newBulk(i + 1)
	}

	w.ticker = time.NewTicker(time.Millisecond)

	go w.run()

	return w
}

func (w *TimingWheel) Stop() {
	close(w.quit)
}

func (w *TimingWheel) After(timeout time.Duration) <-chan struct{} {

	bulk := int(timeout / w.interval)

	if bulk <= 0 {
		ch := make(chan struct{})
		go func() {
			close(ch)
		}()

		return ch
	}

	w.Lock()

	circle := (w.position+bulk)/kTimingBulkSize + w.circle

	idx := (w.position + bulk) % kTimingBulkSize

	b := w.bulks[idx]

	ch := b.after(circle)

	w.Unlock()

	return ch
}

func (w *TimingWheel) run() {
	for {
		select {
		case <-w.ticker.C:
			w.onTicker()
		case <-w.quit:
			w.ticker.Stop()
			return
		}
	}
}

func (w *TimingWheel) onTicker() {
	w.Lock()
	bulk := w.bulks[w.position]

	w.position = (w.position + 1)
	if w.position >= kTimingBulkSize {
		w.circle++
		w.position = w.position % kTimingBulkSize
	}

	bulk.onTicker(w.circle)

	w.Unlock()
}
