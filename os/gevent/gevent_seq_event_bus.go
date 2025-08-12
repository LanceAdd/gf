package gevent

import (
	"sync"

	"github.com/gogf/gf/v2/container/garray"
	"github.com/gogf/gf/v2/container/gmap"
	"github.com/gogf/gf/v2/container/gtype"
)

type topicProcessor struct {
	topic        string
	eventBus     *SeqEventBus
	ch           chan Event
	closed       *gtype.Bool
	processors   *garray.SortedArray
	factoryFunc  EventFactoryFunc
	factoryMutex sync.RWMutex
	startOnce    sync.Once
	closeOnce    sync.Once
}

type handlerProcessor struct {
	id          int64
	priority    Priority
	topic       string
	handlerFunc HandlerFunc
	recoverFunc RecoverFunc
	errorFunc   ErrorFunc
}

type SeqEventBusOption struct {
	QueueSize  int
	WorkerSize int
	OnError    ErrorModel
}

type SeqEventBus struct {
	topics    *gmap.StrAnyMap
	counter   *gtype.Int64
	closeOnce sync.Once
	closed    *gtype.Bool
	option    SeqEventBusOption
	wg        sync.WaitGroup
}

func (tp *topicProcessor) unsetFactoryFunc() {
	tp.factoryMutex.Lock()
	defer tp.factoryMutex.Unlock()
	tp.factoryFunc = nil
}

func (tp *topicProcessor) setFactoryFunc(factoryFunc EventFactoryFunc) {
	tp.factoryMutex.Lock()
	defer tp.factoryMutex.Unlock()
	tp.factoryFunc = factoryFunc
}

func (tp *topicProcessor) getFactoryFunc() EventFactoryFunc {
	tp.factoryMutex.RLock()
	defer tp.factoryMutex.RUnlock()
	return tp.factoryFunc
}

func (tp *topicProcessor) factoryEvent(topic string, params map[string]any, errModel ErrorModel, execModel ExecModel) Event {
	factoryFunc := tp.getFactoryFunc()
	if factoryFunc == nil {
		return BaseEventFactoryFunc(topic, params, errModel, execModel)
	}
	return factoryFunc(topic, params, errModel, execModel)
}

func (tp *topicProcessor) filterHandlerProcessors() []*handlerProcessor {
	eventProcessors := make([]*handlerProcessor, 0)
	tp.processors.Iterator(func(k int, v interface{}) bool {
		processor := v.(*handlerProcessor)
		eventProcessors = append(eventProcessors, processor)
		return true
	})
	return eventProcessors
}

func (tp *topicProcessor) execute(event Event, processor *handlerProcessor) error {
	if processor.recoverFunc == nil {
		wrapper := func(e Event, handlerFunc HandlerFunc) error {
			err := handlerFunc(e)
			if err != nil && processor.errorFunc != nil {
				return processor.errorFunc(e, err)
			}
			return err
		}
		return wrapper(event, processor.handlerFunc)
	}
	wrapper := func(e Event, handlerFunc HandlerFunc, recoverFunc RecoverFunc) (err error) {
		defer func() {
			if r := recover(); r != nil {
				recoverFunc(e, r)
				return
			}
			if err != nil && processor.errorFunc != nil {
				err = processor.errorFunc(e, err)
			}
		}()
		err = handlerFunc(e)
		return err
	}
	return wrapper(event, processor.handlerFunc, processor.recoverFunc)
}

func (tp *topicProcessor) clear() {
	tp.unsetFactoryFunc()
	tp.processors.Clear()
}
func (tp *topicProcessor) close() {
	tp.closeOnce.Do(func() {
		close(tp.ch)
		tp.closed.Set(true)
	})
}
func (tp *topicProcessor) asyncProcess() {
	tp.eventBus.wg.Add(1)
	go func() {
		defer func() {
			tp.clear()
			tp.eventBus.wg.Done()
		}()
		for event := range tp.ch {
			if tp.processors.IsEmpty() {
				continue
			}
			handlerProcessors := tp.filterHandlerProcessors()
			if event.GetExecModel() == Seq {
				for _, processor := range handlerProcessors {
					err := tp.execute(event, processor)
					if err != nil {
						if event.GetErrorModel() == Stop {
							return
						}
					}
				}
			} else {
				workerSize := tp.eventBus.option.WorkerSize
				if workerSize <= 0 {
					workerSize = len(handlerProcessors)
				}
				semaphore := make(chan struct{}, workerSize)
				var wg sync.WaitGroup

				for _, processor := range handlerProcessors {
					wg.Add(1)
					go func(e Event, processor *handlerProcessor) {
						defer wg.Done()
						semaphore <- struct{}{}
						defer func() { <-semaphore }()
						err := tp.execute(e, processor)
						if err != nil {
							if event.GetErrorModel() == Stop {
								return
							}
						}
					}(event, processor)
				}
				wg.Wait()
			}
		}
	}()
}

func NewSeqEventBus(options ...SeqEventBusOption) *SeqEventBus {
	option := SeqEventBusOption{
		QueueSize:  100,
		WorkerSize: 10,
		OnError:    Ignore,
	}
	if len(options) > 0 {
		option = options[0]
	}
	return &SeqEventBus{
		topics:  gmap.NewStrAnyMap(true),
		counter: gtype.NewInt64(),
		closed:  gtype.NewBool(),
		option:  option,
	}
}

func (s *SeqEventBus) RegisterFactoryFunc(topic string, factoryFunc EventFactoryFunc) (bool, error) {
	if s.closed.Val() {
		return false, EventBusClosedError
	}
	if topic == "" {
		return false, TopicEmptyError
	}
	value := s.topics.GetOrSetFunc(topic, func() interface{} {
		return s.initTopicProcessor(topic)
	})
	tp := value.(*topicProcessor)
	if tp.closed.Val() {
		return false, EventBusClosedError
	}
	tp.setFactoryFunc(factoryFunc)
	return true, nil
}

func (s *SeqEventBus) UnRegisterFactoryFunc(topic string) (bool, error) {
	if s.closed.Val() {
		return false, EventBusClosedError
	}
	if topic == "" {
		return false, TopicEmptyError
	}
	value := s.topics.Get(topic)
	if value == nil {
		return false, SubscriberEmptyError
	}
	tp := value.(*topicProcessor)
	if tp.closed.Val() {
		return false, EventBusClosedError
	}
	tp.unsetFactoryFunc()
	return true, nil
}

func (s *SeqEventBus) Publish(topic string, params map[string]any, errModel ErrorModel, execModel ExecModel) (bool, error) {
	if s.closed.Val() {
		return false, EventBusClosedError
	}
	if topic == "" {
		return false, TopicEmptyError
	}
	v := s.topics.Get(topic)
	if v == nil {
		return false, SubscriberEmptyError
	}
	processor := v.(*topicProcessor)
	if processor.closed.Val() {
		return false, EventBusClosedError
	}
	event := processor.factoryEvent(topic, params, errModel, execModel)
	select {
	case processor.ch <- event:
		return true, nil
	default:
		return false, ChannelFullError
	}
}

func (s *SeqEventBus) initTopicProcessor(topic string) *topicProcessor {
	processor := &topicProcessor{
		topic:    topic,
		eventBus: s,
		ch:       make(chan Event, s.option.QueueSize),
		closed:   gtype.NewBool(),
		processors: garray.NewSortedArray(func(a, b interface{}) int {
			ha := a.(*handlerProcessor)
			hb := b.(*handlerProcessor)
			if ha.priority == hb.priority {
				return int(ha.id - hb.id)
			} else {
				return int(hb.priority - ha.priority)
			}
		}, true),
	}
	processor.startOnce.Do(processor.asyncProcess)
	return processor
}

func (s *SeqEventBus) PublishEvent(event Event) (bool, error) {
	if s.closed.Val() {
		return false, EventBusClosedError
	}
	if event == nil {
		return false, EventNilError
	}
	topic := event.GetTopic()
	if topic == "" {
		return false, TopicEmptyError
	}
	v := s.topics.Get(topic)
	if v == nil {
		return false, SubscriberEmptyError
	}
	processor := v.(*topicProcessor)
	if processor.closed.Val() {
		return false, EventBusClosedError
	}
	select {
	case processor.ch <- event:
		return true, nil
	default:
		return false, ChannelFullError
	}
}

func (s *SeqEventBus) Subscribe(topic string, handlerFunc HandlerFunc, errorFunc ErrorFunc, recoverFunc RecoverFunc, priorities ...Priority) (*SeqEventBusSubscriber, error) {
	if s.closed.Val() {
		return nil, EventBusClosedError
	}
	if topic == "" {
		return nil, TopicEmptyError
	}
	if handlerFunc == nil {
		return nil, HandlerNilError
	}
	priority := PriorityNormal
	if len(priorities) > 0 {
		priority = priorities[0]
	}
	value := s.topics.GetOrSetFunc(topic, func() interface{} {
		return s.initTopicProcessor(topic)
	})
	processor := value.(*topicProcessor)
	if processor.closed.Val() {
		return nil, EventBusClosedError
	}
	handlerId := s.counter.Add(1)
	handler := &handlerProcessor{
		id:          handlerId,
		topic:       topic,
		handlerFunc: handlerFunc,
		errorFunc:   errorFunc,
		recoverFunc: recoverFunc,
		priority:    priority,
	}
	processor.processors.Add(handler)
	return &SeqEventBusSubscriber{
		topic:        topic,
		eventBus:     s,
		handler:      handler,
		unsubscribed: gtype.NewBool(),
	}, nil
}
func (s *SeqEventBus) UnSubscribe(topic string, processor *handlerProcessor) (bool, error) {
	if s.closed.Val() {
		return false, EventBusClosedError
	}
	value := s.topics.Get(topic)
	if value == nil {
		return false, SubscriberEmptyError
	}
	tp := value.(*topicProcessor)
	if tp.closed.Val() {
		return false, EventBusClosedError
	}
	res := tp.processors.RemoveValue(processor)
	if res {
		tp.processors.LockFunc(func(array []interface{}) {
			if len(array) == 0 {
				s.topics.Remove(topic)
				tp.close()
			}
		})
		return true, nil
	}
	return res, NoHandlerError
}

func (s *SeqEventBus) Close() {
	s.closeOnce.Do(func() {
		s.closed.Set(true)
		s.topics.LockFunc(func(m map[string]interface{}) {
			for _, value := range m {
				tp := value.(*topicProcessor)
				tp.close()
			}
		})
		s.wg.Wait()
	})
}

func (s *SeqEventBus) IsClosed() bool {
	return s.closed.Val()
}

type SeqEventBusSubscriber struct {
	topic        string
	once         sync.Once
	eventBus     *SeqEventBus
	handler      *handlerProcessor
	unsubscribed *gtype.Bool
}

func (sub *SeqEventBusSubscriber) GetTopic() string {
	return sub.topic
}
func (sub *SeqEventBusSubscriber) GetEventBus() *SeqEventBus {
	return sub.eventBus
}

func (sub *SeqEventBusSubscriber) UnSubscribe() (bool, error) {
	var (
		res bool
		err error
	)
	sub.once.Do(func() {
		res, err = sub.eventBus.UnSubscribe(sub.topic, sub.handler)
		if err == nil {
			sub.unsubscribed.Set(true)
		}
	})
	if sub.unsubscribed.Val() {
		return true, nil
	}
	return res, err
}
