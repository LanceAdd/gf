module github.com/gogf/gf/v2/net/gtransport/example/gtcp_echo

go 1.23.0

require (
	github.com/gogf/gf/v2 v2.0.0
	github.com/gogf/gf/v2/net/gtransport v0.0.0
)

require (
	github.com/emirpasic/gods/v2 v2.0.0-alpha // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.25.0 // indirect
)

replace (
	github.com/gogf/gf/v2 => ../../../../
	github.com/gogf/gf/v2/net/gtransport => ../../
)
