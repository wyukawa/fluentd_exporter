# fluentd Exporter for Prometheus
Exports fluentd result for Prometheus consumption.

How to build  
(Dependency libraries have been included in the `vendor` directory using [dep](https://github.com/golang/dep))
```
make
```

Help on flags of fluentd_exporter:
```
  -web.listen-address string
    	Address on which to expose metrics and web interface. (default ":9224")
  -web.telemetry-path string
    	Path under which to expose metrics. (default "/metrics")
```
