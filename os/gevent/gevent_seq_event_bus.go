package gevent

import (
	"github.com/gogf/gf/v2/container/garray"
	"github.com/gogf/gf/v2/container/gmap"
	"github.com/gogf/gf/v2/container/gtype"
	"github.com/gogf/gf/v2/util/gutil"
	"sync"
)

type topicProcessor struct {
	topic      string
	ch         chan Event
	processors *garray.SortedArray
}

func (p *topicProcessor) filterEventProcessors() []*eventProcessor {
	eventProcessors := make([]*eventProcessor, 0)
	p.processors.Iterator(func(k int, v interface{}) bool {
		processor := v.(*eventProcessor)
		eventProcessors = append(eventProcessors, processor)
		return true
	})
	return eventProcessors
}

func (p *topicProcessor) executeWithRecover(event Event, processor *eventProcessor) error {
	wrapper := func(e Event, handlerFunc HandlerFunc, recoverFunc RecoverFunc) error {
		defer func() {
			if r := recover(); r != nil {
				recoverFunc(e, r)
			}
		}()
		return handlerFunc(e)
	}
	return wrapper(event, processor.handlerFunc, processor.recoverFunc)
}

func (p *topicProcessor) executeEvey(event Event, processor *eventProcessor) error {
	return processor.handlerFunc(event)
}
func (p *topicProcessor) process() {
	go func() {
		for event := range p.ch {
			if p.processors.IsEmpty() {
				continue
			}
			eventProcessors := p.filterEventProcessors()
			if event.GetHandleStrategy() == Seq {
				for _, processor := range eventProcessors {
					err := p.executeWithRecover(event, processor)
					if err != nil {
						if event.GetErrorModel() == Stop {
							return
						}
					}
				}
			} else {

			}
		}
	}()
}

type eventProcessor struct {
	id          int64
	priority    Priority
	topic       string
	handlerFunc HandlerFunc
	recoverFunc RecoverFunc
}

type SeqOption struct {
	Model      PublishModel
	QueueSize  int
	WorkerSize int
	OnError    ErrorModel
}

type SeqEventBus struct {
	topics    *gmap.StrAnyMap
	factory   *gmap.StrAnyMap
	counter   *gtype.Int64
	closeOnce sync.Once
	closed    *gtype.Bool
	option    SeqOption
	waitGroup sync.WaitGroup
}

func NewSeqEventBus(options ...SeqOption) *SeqEventBus {
	option := SeqOption{
		Model:      DropIfFull,
		QueueSize:  100,
		WorkerSize: 10,
		OnError:    Ignore,
	}
	if len(options) > 0 {
		option = options[0]
	}
	return &SeqEventBus{
		topics:  gmap.NewStrAnyMap(true),
		factory: gmap.NewStrAnyMap(true),
		counter: gtype.NewInt64(),
		closed:  gtype.NewBool(),
		option:  option,
	}
}

func (s *SeqEventBus) RegisterEventFactoryFunc(topic string, factoryFunc EventFactoryFunc) (bool, error) {
	if gutil.IsEmpty(topic) {
		return false, TopicEmptyError
	}
	if factoryFunc == nil {
		return false, FactoryFuncIsNilError
	}
	s.factory.Set(topic, factoryFunc)
	return true, nil
}

func (s *SeqEventBus) UnRegisterEventFactoryFunc(topic string) (bool, error) {
	if gutil.IsEmpty(topic) {
		return false, TopicEmptyError
	}
	s.factory.Remove(topic)
	return true, nil
}

func (s *SeqEventBus) factoryEvent(topic string, params map[string]any, execModel ExecModel) Event {
	value := s.factory.Get(topic)
	if value != nil {
		if f, ok := value.(EventFactoryFunc); ok {
			return f(topic, params)
		}
	}
	return BaseEventFactoryFunc(topic, params)
}

func (s *SeqEventBus) Publish(topic string, params map[string]any, execModels ...ExecModel) (bool, error) {
	if topic == "" {
		return false, TopicEmptyError
	}
	v := s.topics.Get(topic)
	if v == nil {
		return false, SubscriberEmptyError
	}
	execModel := Seq
	if len(execModels) > 0 {
		execModel = execModels[0]
	}
	event := s.factoryEvent(topic, params, execModel)
	processor := v.(*topicProcessor)
	select {
	case processor.ch <- event:
		return true, nil
	default:
		return false, ChannelFullError
	}
}

func (s *SeqEventBus) PublishEvent(event Event) (bool, error) {
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
	select {
	case processor.ch <- event:
		return true, nil
	default:
		return false, ChannelFullError
	}
}

func (s *SeqEventBus) Subscribe(topic string, handlerFunc HandlerFunc, priority ...Priority) *Subscriber {
	if topic == "" {
		return nil
	}
	if handlerFunc ==
}
