package gevent

import (
	"github.com/gogf/gf/v2/errors/gerror"
)

var (
	EventBusClosedError   = gerror.New("event bus closed")
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

type ExecModel int

const (
	Seq ExecModel = iota
	Parallel
)
