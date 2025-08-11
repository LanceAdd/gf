package gevent

import (
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
	Model      PublishModel
	EnableLock bool
	QueueSize  int
	WorkerSize int
	OnError    ErrorModel
}

type EventManager struct {
	mu        sync.RWMutex
	topics    *gmap.StrAnyMap
	factory   *gmap.StrAnyMap
	counter   *gtype.Int64
	closeOnce sync.Once
	closed    *gtype.Bool
	ch        chan Event
	options   ManagerOption
	waitGroup sync.WaitGroup
}

func New(options ...ManagerOption) *EventManager {
	option := ManagerOption{
		EnableLock: false,
		QueueSize:  100,
		WorkerSize: 10,
	}
	if len(options) > 0 {
		option = options[0]
	}
	manager := EventManager{
		topics:  gmap.NewStrAnyMap(true),
		factory: gmap.NewStrAnyMap(true),
		counter: gtype.NewInt64(),
		closed:  gtype.NewBool(),
		ch:      make(chan Event, option.QueueSize),
		options: option,
	}
	manager.startConsumer()
	return &manager
}

func (em *EventManager) RegisterEventFactoryFunc(topic string, factoryFunc EventFactoryFunc) (bool, error) {
	if gutil.IsEmpty(topic) {
		return false, TopicEmptyError
	}

	if factoryFunc == nil {
		return false, FactoryFuncIsNilError
	}
	em.factory.Set(topic, factoryFunc)
	return true, nil
}

func (em *EventManager) UnRegisterEventFactoryFunc(topic string) (bool, error) {
	if gutil.IsEmpty(topic) {
		return false, TopicEmptyError
	}
	em.factory.Remove(topic)
	return true, nil
}

func (em *EventManager) filterHandlers(topic string) []*EventHandler {
	value := em.topics.Get(topic)
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

func (em *EventManager) factoryEvent(topic string, params map[string]any) Event {
	value := em.factory.Get(topic)
	if value != nil {
		if f, ok := value.(EventFactoryFunc); ok {
			return f(topic, params)
		}
	}
	return BaseEventFactoryFunc(topic, params)
}

func (em *EventManager) executeEventBlock(event Event) (bool, error) {
	handlers := em.filterHandlers(event.GetTopic())
	for _, handler := range handlers {
		var err error
		e := event.Clone()
		if handler.recoverFunc != nil {
			err = em.executeHandlerWithRecover(e, handler)
		} else {
			err = em.executeHandler(e, handler)
		}
		if err != nil {
			if e.GetErrorStrategy() == ErrorStrategyStop {
				return false, err
			} else if em.options.OnError == ErrorStrategyStop {
				return false, err
			} else {
				continue
			}
		}
	}
	return true, nil
}

func (em *EventManager) executeEventParallel(event Event) {
	handlers := em.filterHandlers(event.GetTopic())
	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(e Event, handler *EventHandler) {
			defer wg.Done()
			if handler.recoverFunc != nil {
				_ = em.executeHandlerWithRecover(e, handler)
			} else {
				_ = em.executeHandler(e, handler)
			}
		}(event.Clone(), handler)
	}
	go func() {
		wg.Wait()
	}()
}

func (em *EventManager) executeEventParallelWait(event Event) []error {
	handlers := em.filterHandlers(event.GetTopic())
	var wg sync.WaitGroup
	errCh := make(chan error, len(handlers))
	for _, handler := range handlers {
		wg.Add(1)
		go func(e Event, handler *EventHandler) {
			defer wg.Done()
			var err error
			if handler.recoverFunc != nil {
				err = em.executeHandlerWithRecover(e, handler)
			} else {
				err = em.executeHandler(e, handler)
			}
			errCh <- err
		}(event.Clone(), handler)
	}
	wg.Wait()
	close(errCh)
	var errors []error
	for err := range errCh {
		if err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func (em *EventManager) executeEventChannel(event Event) (bool, error) {
	select {
	case em.ch <- event:
		return true, nil
	default:
		return false, ChannelFullError
	}
}

func (em *EventManager) startConsumer() {
	go func() {
		defer func() {
			em.factory.Clear()
			em.topics.Clear()
		}()
		for range em.options.WorkerSize {
			em.waitGroup.Add(1)
			go func(em *EventManager) {
				defer em.waitGroup.Done()
				for event := range em.ch {
					handlers := em.filterHandlers(event.GetTopic())
					for _, handler := range handlers {
						e := event.Clone()
						if handler.recoverFunc != nil {
							_ = em.executeHandlerWithRecover(e, handler)
						} else {
							_ = em.executeHandler(e, handler)
						}
					}
				}
			}(em)
		}
		em.waitGroup.Wait()
	}()
}

func (em *EventManager) executeHandlerWithRecover(event Event, handler *EventHandler) error {
	wrapper := func(e Event, handlerFunc HandlerFunc, recoverFunc RecoverFunc) error {
		defer func() {
			if r := recover(); r != nil {
				recoverFunc(e, r)
			}
		}()
		return handlerFunc(e)
	}
	return wrapper(event, handler.handlerFunc, handler.recoverFunc)
}

func (em *EventManager) executeHandler(event Event, handler *EventHandler) error {
	return handler.handlerFunc(event)
}

func (em *EventManager) PublishBlock(topic string, params map[string]any) (bool, error) {
	if topic == "" {
		return false, TopicEmptyError
	}
	if em.closed.Val() {
		return false, ClosedError
	}
	event := em.factoryEvent(topic, params)
	return em.executeEventBlock(event)
}

func (em *EventManager) PublishEventBlock(event Event) (bool, error) {
	if event == nil {
		return false, EventNilError
	}
	if event.GetTopic() == "" {
		return false, TopicEmptyError
	}
	if em.closed.Val() {
		return false, ClosedError
	}
	return em.executeEventBlock(event)
}

func (em *EventManager) PublishAsync(topic string, params map[string]any) error {
	if topic == "" {
		return TopicEmptyError
	}
	if em.closed.Val() {
		return ClosedError
	}
	event := em.factoryEvent(topic, params)
	go func(e Event) {
		_, _ = em.executeEventBlock(e)
	}(event)
	return nil
}

func (em *EventManager) PublishEventAsync(event Event) error {
	if event == nil {
		return EventNilError
	}
	if event.GetTopic() == "" {
		return TopicEmptyError
	}
	if em.closed.Val() {
		return ClosedError
	}
	go func(e Event) {
		_, _ = em.executeEventBlock(e)
	}(event)
	return nil
}

func (em *EventManager) PublishParallel(topic string, params map[string]any) error {
	if topic == "" {
		return TopicEmptyError
	}
	if em.closed.Val() {
		return ClosedError
	}
	event := em.factoryEvent(topic, params)
	em.executeEventParallel(event)
	return nil
}

func (em *EventManager) PublishEventParallel(event Event) error {
	if event == nil {
		return EventNilError
	}
	if event.GetTopic() == "" {
		return TopicEmptyError
	}
	if em.closed.Val() {
		return ClosedError
	}
	em.executeEventParallel(event)
	return nil
}

func (em *EventManager) PublishParallelWait(topic string, params map[string]any) []error {
	if topic == "" {
		return []error{TopicEmptyError}
	}
	if em.closed.Val() {
		return []error{ClosedError}
	}
	event := em.factoryEvent(topic, params)
	return em.executeEventParallelWait(event)
}

func (em *EventManager) PublishEventParallelWait(event Event) []error {
	if event == nil {
		return []error{EventNilError}
	}
	if event.GetTopic() == "" {
		return []error{TopicEmptyError}
	}
	if em.closed.Val() {
		return []error{ClosedError}
	}
	return em.executeEventParallelWait(event)
}

func (em *EventManager) PublishChannel(topic string, params map[string]any) (bool, error) {
	if topic == "" {
		return false, TopicEmptyError
	}
	if em.closed.Val() {
		return false, ClosedError
	}
	event := em.factoryEvent(topic, params)
	return em.executeEventChannel(event)
}

func (em *EventManager) PublishEventChannel(event Event) (bool, error) {
	if event == nil {
		return false, EventNilError
	}
	if event.GetTopic() == "" {
		return false, TopicEmptyError
	}
	if em.closed.Val() {
		return false, ClosedError
	}
	return em.executeEventChannel(event)
}

func (em *EventManager) Close() {
	em.closeOnce.Do(func() {
		em.closed.Set(true)
		close(em.ch)
	})
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

func (em *EventManager) unSubscribe(handler *EventHandler) {
	value := em.topics.Get(handler.topic)
	if value == nil {
		return
	}
	handlers := value.(*garray.SortedArray)
	if handlers.IsEmpty() {
		return
	}
	handlers.RemoveValue(handler)
}

func (em *EventManager) SubscribeWithRecover(topic string, handlerFunc HandlerFunc, recoverFunc RecoverFunc, priorities ...Priority) *Subscriber {
	priority := PriorityNormal
	if len(priorities) > 0 {
		priority = priorities[0]
	}
	handlerId := em.counter.Add(1)
	handlerArray := em.topics.GetOrSet(topic, garray.NewSortedArray(func(a, b interface{}) int {
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
		Manager: em,
		handler: handler,
	}
}

func (em *EventManager) Subscribe(topic string, handlerFunc HandlerFunc, priority ...Priority) *Subscriber {
	return em.SubscribeWithRecover(topic, handlerFunc, nil, priority...)
}
