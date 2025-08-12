package gevent

import (
	"github.com/gogf/gf/v2/errors/gerror"
)

var (
	ClosedError           = gerror.New("event manager closed")
	ChannelFullError      = gerror.New("event channel full")
	TopicEmptyError       = gerror.New("topic is empty")
	HandlerNilError       = gerror.New("handler is nil")
	NotFoundError         = gerror.New("not found")
	SubscriberEmptyError  = gerror.New("subscriber is empty")
	NoHandlerError        = gerror.New("no handler")
	EventNilError         = gerror.New("event is empty")
	FactoryFuncIsNilError = gerror.New("factory func is nil")
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

type ErrorModel int

const (
	Stop ErrorModel = iota
	Ignore
)

type PublishModel int

const (
	DropIfFull PublishModel = iota
	CallbackIfFull
)

type ExecModel int

const (
	Seq ExecModel = iota
	Parallel
)

type Event interface {
	SetTopic(topic string)
	GetTopic() string
	SetData(data map[string]any)
	GetData() map[string]any
	SetErrorModel(model ErrorModel)
	GetErrorModel() ErrorModel
	SetHandleStrategy(model ExecModel)
	GetHandleStrategy() ExecModel
	Clone() Event
}

type HandlerFunc func(e Event) error
type RecoverFunc func(e Event, err any)
type ErrorFunc func(e Event, err error) error
type EventFactoryFunc func(topic string, params map[string]any, errorModel ErrorModel, execModel ExecModel) Event

type EventBus interface {
	Publish(topic string, params map[string]any) (bool, error)
	SubscribeWithRecover(topic string, handlerFunc HandlerFunc, recoverFunc RecoverFunc, priority ...Priority) *Subscriber
	Subscribe(topic string, handlerFunc HandlerFunc, priority ...Priority) *Subscriber
	unSubscribe(handler *EventHandler)
	Close()
}

type EventSubscriber interface {
}

type ManagerCloser interface {
	Close()
}

type EventUnSubscriber interface {
	unSubscribe(handler *EventHandler)
}
