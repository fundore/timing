package timing

import (
	"fmt"
	"testing"
	"time"
)

func TestTiming(t *testing.T) {
	timing := NewTimingWheel()

	ch := make(chan struct{})

	delta := time.Millisecond

	go func() {
		fmt.Println(time.Now().Format(time.StampMilli))

		select {
		case <-timing.After(delta):
		}

		fmt.Println(time.Now().Format(time.StampMilli))

		for {

			delta += 2

			select {
			case <-timing.After(delta):
			case <-timing.After(time.Second):
			}
			fmt.Println(time.Now().Format(time.StampMilli))
		}
	}()

	select {
	case <-ch:
	}

	timing.Stop()

	fmt.Println("test exit")
}
