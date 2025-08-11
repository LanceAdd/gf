package gevent

import (
	"sync"
)

type BaseSubscriber struct {
	Topic   string
	once    sync.Once
	Core    *EventBus
	handler *EventHandler
}
