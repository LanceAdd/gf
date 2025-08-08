package gevent

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/container/garray"
	"github.com/gogf/gf/v2/container/gmap"
	"github.com/gogf/gf/v2/container/gtype"
	"github.com/gogf/gf/v2/util/gutil"
)

type EventHandler struct {
	id          int64
	topic       string
	handlerFunc HandlerFunc
	priority    Priority
	recoverFunc RecoverFunc
}

type ManagerOption struct {
	EnableLock bool
	QueueSize  int
	WorkerSize int
	OnError    ErrorStrategy
}

type EventManager struct {
	mu      sync.RWMutex
	topics  *gmap.StrAnyMap
	factory *gmap.StrAnyMap
	counter *gtype.Int64
	closed  *gtype.Bool
	chOnce  sync.Once
	ch      chan Event
	options ManagerOption
}

func New(options ...ManagerOption) *EventManager {
	option := ManagerOption{
		EnableLock: false,
		QueueSize:  100,
		WorkerSize: 10,
		OnError:    ErrorStrategyStop,
	}
	if len(options) > 0 {
		option = options[0]
	}
	return &EventManager{
		topics:  gmap.NewStrAnyMap(true),
		factory: gmap.NewStrAnyMap(true),
		counter: gtype.NewInt64(),
		closed:  gtype.NewBool(),
		options: option,
	}
}

type BaseEvent struct {
	Topic string
	Data  map[string]any
}

func (b *BaseEvent) SetTopic(topic string) {
	b.Topic = topic
}

func (b *BaseEvent) GetTopic() string {
	return b.Topic
}

func (b *BaseEvent) SetData(data map[string]any) {
	b.Data = data
}

func (b *BaseEvent) GetData() map[string]any {
	return b.Data
}

func (e *EventManager) RegisterEventFactoryFunc(topic string, factoryFunc EventFactoryFunc) (bool, error) {
	if gutil.IsEmpty(topic) {
		return false, TopicEmptyError
	}

	if factoryFunc == nil {
		return false, FactoryFuncIsNilError
	}
	e.factory.Set(topic, factoryFunc)
	return true, nil
}

func (e *EventManager) factoryEvent(topic string, params map[string]any) (bool, error) {

	return true, nil
}

func NewEvent(topic string, params map[string]any) *BaseEvent {
	return &BaseEvent{
		Topic: topic,
		Data:  params,
	}
}

func (e *EventManager) PublishBlock(topic string, params map[string]any) (bool, error) {
	if topic == "" {
		return false, TopicEmptyError
	}
	if e.closed.Val() {
		return false, ManagerClosedError
	}
	handlers := e.filterHandlers(topic)
	for _, handler := range handlers {
		var err error
		if handler.recoverFunc != nil {
			err = e.executeHandlerWithRecover(topic, params, handler)
		} else {
			err = e.executeHandler(topic, params, handler)
		}
		if err != nil && e.options.OnError == ErrorStrategyStop {
			return false, err
		}
	}
	return true, nil
}

func (e *EventManager) filterHandlers(topic string) []*EventHandler {
	value := e.topics.Get(topic)
	if value == nil {
		return nil
	}
	handlers := value.(*garray.SortedArray)
	if handlers.IsEmpty() {
		return nil
	}
	handlersSlice := make([]*EventHandler, handlers.Len())
	handlers.Iterator(func(k int, v interface{}) bool {
		handlersSlice[k] = v.(*EventHandler)
		return true
	})
	return handlersSlice
}

func (e *EventManager) executeHandlerWithRecover(topic string, params map[string]any, handler *EventHandler) error {
	wrapper := func(e Event, handlerFunc HandlerFunc, recoverFunc RecoverFunc) error {
		defer func() {
			if r := recover(); r != nil {
				recoverFunc(e, r)
			}
		}()
		return handlerFunc(e)
	}
	return wrapper(NewEvent(topic, params), handler.handlerFunc, handler.recoverFunc)
}

func (e *EventManager) executeHandler(topic string, params map[string]any, handler *EventHandler) error {
	return handler.handlerFunc(NewEvent(topic, params))
}

func (e *EventManager) PublishAsync(topic string, params map[string]any) {
	if topic == "" {
		return
	}
	if e.closed.Val() {
		return
	}
	go func() {
		_, _ = e.PublishBlock(topic, params)
	}()
}

func (e *EventManager) PublishParallel(topic string, params map[string]any) {
	if topic == "" {
		return
	}
	if e.closed.Val() {
		return
	}
	handlers := e.filterHandlers(topic)
	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(handler *EventHandler) {
			defer wg.Done()
			if handler.recoverFunc != nil {
				_ = e.executeHandlerWithRecover(topic, params, handler)
			} else {
				_ = e.executeHandler(topic, params, handler)
			}
		}(handler)
	}
	go func() {
		wg.Wait()
	}()
}

func (e *EventManager) PublishParallelWait(topic string, params map[string]any) []error {
	if topic == "" {
		return []error{TopicEmptyError}
	}
	if e.closed.Val() {
		return []error{ManagerClosedError}
	}
	handlers := e.filterHandlers(topic)
	var wg sync.WaitGroup
	ch := make(chan error, len(handlers))
	for _, handler := range handlers {
		wg.Add(1)
		go func(handler *EventHandler) {
			defer wg.Done()
			var err error
			if handler.recoverFunc != nil {
				err = e.executeHandlerWithRecover(topic, params, handler)
			} else {
				err = e.executeHandler(topic, params, handler)
			}
			ch <- err
		}(handler)
	}
	wg.Wait()
	close(ch)
	var errors []error
	for err := range ch {
		if err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func (e *EventManager) PublishChannel(topic string, params map[string]any) (bool, error) {
	if topic == "" {
		return false, TopicEmptyError
	}
	if e.closed.Val() {
		return false, ManagerClosedError
	}
	e.chOnce.Do(func() {
		e.ch = make(chan Event, e.options.QueueSize)
		e.StartConsumer()
	})
	select {
	case e.ch <- NewEvent(topic, params):
		return true, nil
	default:
		return false, ManagerChannelFullError
	}
}

func (e *EventManager) StartConsumer() {
	for range e.options.WorkerSize {
		go func() {
			for event := range e.ch {
				handlers := e.filterHandlers(event.GetTopic())
				for _, handler := range handlers {
					if handler.recoverFunc != nil {
						_ = e.executeHandlerWithRecover(event.GetTopic(), event.GetData(), handler)
					} else {
						_ = e.executeHandler(event.GetTopic(), event.GetData(), handler)
					}
				}
			}
		}()
	}
}

func (e *EventManager) PublishCtxAsync(ctx context.Context, topic string, params map[string]any) {
	if e.closed.Val() {
		return
	}
	go func() {
		handlers := e.filterHandlers(topic)
		for _, handler := range handlers {
			select {
			case <-ctx.Done():
				return
			default:
				if handler.recoverFunc != nil {
					_ = e.executeHandlerWithRecover(topic, params, handler)
				} else {
					_ = e.executeHandler(topic, params, handler)
				}
			}
		}
	}()
}

func (e *EventManager) Close() {
	e.closed.Set(true)
	close(e.ch)
	e.topics.Clear()
}

type Subscriber struct {
	Topic   string
	once    sync.Once
	Manager *EventManager
	handler *EventHandler
}

func (s *Subscriber) UnSubscribe() {
	s.once.Do(func() {
		s.Manager.unSubscribe(s.handler)
	})
}

func (e *EventManager) unSubscribe(handler *EventHandler) {
	value := e.topics.Get(handler.topic)
	if value == nil {
		return
	}
	handlers := value.(*garray.SortedArray)
	if handlers.IsEmpty() {
		return
	}
	handlers.RemoveValue(handler)
}

func (e *EventManager) SubscribeWithRecover(topic string, handlerFunc HandlerFunc, recoverFunc RecoverFunc, priorities ...Priority) *Subscriber {
	priority := PriorityNormal
	if len(priorities) > 0 {
		priority = priorities[0]
	}
	handlerId := e.counter.Add(1)
	handlerArray := e.topics.GetOrSet(topic, garray.NewSortedArray(func(a, b interface{}) int {
		ha := a.(*EventHandler)
		hb := b.(*EventHandler)
		if ha.priority == hb.priority {
			return int(ha.id - hb.id)
		} else {
			return int(hb.priority - ha.priority)
		}
	}, true))
	handlers := handlerArray.(*garray.SortedArray)
	handler := &EventHandler{
		id:          handlerId,
		topic:       topic,
		handlerFunc: handlerFunc,
		priority:    priority,
		recoverFunc: recoverFunc,
	}
	handlers.Add(handler)
	return &Subscriber{
		Topic:   topic,
		Manager: e,
		handler: handler,
	}
}

func (e *EventManager) Subscribe(topic string, handlerFunc HandlerFunc, priority ...Priority) *Subscriber {
	return e.SubscribeWithRecover(topic, handlerFunc, nil, priority...)
}
