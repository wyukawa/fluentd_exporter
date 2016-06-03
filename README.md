# fluentd Exporter for Prometheus
Exports fluentd result for Prometheus consumption.

How to build
```
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/log
go get github.com/prometheus/procfs
go build fluentd_exporter.go
```

Help on flags of fluentd_exporter:
```
  -web.listen-address string
    	Address on which to expose metrics and web interface. (default ":9224")
  -web.telemetry-path string
    	Path under which to expose metrics. (default "/metrics")
```
