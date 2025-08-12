package gevent

type Event interface {
	SetTopic(topic string)
	GetTopic() string
	SetData(data map[string]any)
	GetData() map[string]any
	SetErrorModel(model ErrorModel)
	GetErrorModel() ErrorModel
	SetExecModel(model ExecModel)
	GetExecModel() ExecModel
	Clone() Event
}

type HandlerFunc func(e Event) error
type RecoverFunc func(e Event, err any)
type ErrorFunc func(e Event, err error) error
type EventFactoryFunc func(topic string, params map[string]any, errorModel ErrorModel, execModel ExecModel) Event
