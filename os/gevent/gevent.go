package gevent

import (
	"github.com/gogf/gf/v2/errors/gerror"
)

var (
	ManagerClosedError      = gerror.New("event manager closed")
	ManagerChannelFullError = gerror.New("event channel full")
	TopicEmptyError         = gerror.New("topic is empty")
	EventEmptyError         = gerror.New("event is empty")
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
	SetErrorStrategy(strategy ErrorStrategy)
	GetErrorStrategy() ErrorStrategy
	Clone() Event
}

type HandlerFunc func(e Event) error
type RecoverFunc func(e Event, err any)
type EventFactoryFunc func(topic string, params map[string]any) Event

var BaseEventFactory = func(topic string, params map[string]any) *BaseEvent {
	return &BaseEvent{
		Topic:   topic,
		Data:    params,
		OnError: ErrorStrategyIgnore,
	}
}

type EventPublisher interface {
	PublishBlock(topic string, params map[string]any) (bool, error)
	PublishEventBlock(event Event) (bool, error)
	PublishAsync(topic string, params map[string]any) error
	PublishEventAsync(event Event) error
	PublishParallel(topic string, params map[string]any) error
	PublishEventParallel(event Event) error
	PublishParallelWait(topic string, params map[string]any) []error
	PublishEventParallelWait(event Event) []error
	PublishChannel(topic string, params map[string]any) (bool, error)
	PublishEventChannel(event Event) (bool, error)
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

var DefaultEventManager = New()
