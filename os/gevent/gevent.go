package gevent

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
)

var (
	ManagerClosedError      = gerror.New("event manager closed")
	ManagerChannelFullError = gerror.New("event channel full")
	TopicEmptyError         = gerror.New("topic is empty")
	FactoryFuncIsNilError   = gerror.New("factory func is nil")
)

type Priority int

const (
	PriorityNone Priority = iota
	PriorityLow
	PriorityNormal
	PriorityHigh
	PriorityUrgent
	PriorityImmediate
)

type ErrorStrategy int

const (
	ErrorStrategyIgnore ErrorStrategy = iota
	ErrorStrategyStop
)

type Event interface {
	SetTopic(topic string)
	GetTopic() string
	SetData(data map[string]any)
	GetData() map[string]any
}

type HandlerFunc func(e Event) error
type RecoverFunc func(e Event, err any)
type EventFactoryFunc func(topic string, params map[string]any) Event

type EventPublisher interface {
	PublishBlock(topic string, params map[string]any) (bool, error)
	PublishAsync(topic string, params map[string]any)
	PublishParallel(topic string, params map[string]any)
	PublishParallelWait(topic string, params map[string]any) []error
	PublishChannel(topic string, params map[string]any) (bool, error)
	PublishCtxAsync(ctx context.Context, topic string, params map[string]any)
}

type EventSubscriber interface {
	SubscribeWithRecover(topic string, handlerFunc HandlerFunc, recoverFunc RecoverFunc, priority ...Priority) *Subscriber
	Subscribe(topic string, handlerFunc HandlerFunc, priority ...Priority) *Subscriber
}

type ManagerCloser interface {
	Close()
}

type EventUnSubscriber interface {
	unSubscribe(handler *EventHandler)
}
