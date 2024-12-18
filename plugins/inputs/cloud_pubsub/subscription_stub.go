package cloud_pubsub

import (
	"context"
	"sync"
	"time"
)

type stubSub struct {
	id       string
	messages chan *testMsg
	receiver receiveFunc
}

func (s *stubSub) ID() string {
	return s.id
}

func (s *stubSub) Receive(ctx context.Context, f func(context.Context, message)) error {
	return s.receiver(ctx, f)
}

type receiveFunc func(ctx context.Context, f func(context.Context, message)) error

func testMessagesError(expectedErr error) receiveFunc {
	return func(context.Context, func(context.Context, message)) error {
		return expectedErr
	}
}

func testMessagesReceive(s *stubSub) receiveFunc {
	return func(ctx context.Context, f func(context.Context, message)) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case m := <-s.messages:
				f(ctx, m)
			}
		}
	}
}

type testMsg struct {
	id          string
	value       string
	attributes  map[string]string
	publishTime time.Time

	tracker *testTracker
}

func (tm *testMsg) Ack() {
	tm.tracker.ack()
}

func (tm *testMsg) Nack() {
	tm.tracker.nack()
}

func (tm *testMsg) ID() string {
	return tm.id
}

func (tm *testMsg) Data() []byte {
	return []byte(tm.value)
}

func (tm *testMsg) Attributes() map[string]string {
	return tm.attributes
}

func (tm *testMsg) PublishTime() time.Time {
	return tm.publishTime
}

type testTracker struct {
	sync.Mutex
	*sync.Cond

	numAcks  int
	numNacks int
}

func (t *testTracker) waitForAck(num int) {
	t.Lock()
	if t.Cond == nil {
		t.Cond = sync.NewCond(&t.Mutex)
	}
	for t.numAcks < num {
		t.Wait()
	}
	t.Unlock()
}

func (t *testTracker) ack() {
	t.Lock()
	defer t.Unlock()

	t.numAcks++
}

func (t *testTracker) nack() {
	t.Lock()
	defer t.Unlock()

	t.numNacks++
}
