package gevent

var BaseEventFactoryFunc = func(topic string, params map[string]any, errorModel ErrorModel, execModel ExecModel) Event {
	return &BaseEvent{
		Topic:      topic,
		Data:       params,
		errorModel: errorModel,
		execModel:  execModel,
	}
}

type BaseEvent struct {
	Topic      string
	Data       map[string]any
	errorModel ErrorModel
	execModel  ExecModel
}

func (be *BaseEvent) SetTopic(topic string) {
	be.Topic = topic
}

func (be *BaseEvent) GetTopic() string {
	return be.Topic
}

func (be *BaseEvent) SetData(data map[string]any) {
	be.Data = data
}

func (be *BaseEvent) GetData() map[string]any {
	return be.Data
}
func (be *BaseEvent) SetErrorModel(model ErrorModel) {
	be.errorModel = model
}
func (be *BaseEvent) GetErrorModel() ErrorModel {
	return be.errorModel
}

func (be *BaseEvent) GetHandleStrategy() ExecModel {
	return be.execModel
}

func (be *BaseEvent) SetHandleStrategy(execModel ExecModel) {
	be.execModel = execModel
}

func (be *BaseEvent) Clone() Event {
	return &BaseEvent{
		Topic:      be.Topic,
		Data:       be.Data,
		errorModel: be.errorModel,
		execModel:  be.execModel,
	}
}
