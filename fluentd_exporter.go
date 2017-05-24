package main

import (
	"flag"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/log"
	"github.com/prometheus/procfs"
)

const (
	namespace = "fluentd"
)

var (
	listenAddress     = flag.String("web.listen-address", ":9224", "Address on which to expose metrics and web interface.")
	metricsPath       = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	fluentdConfRegexp = regexp.MustCompile("\\W([\\w\\.]+\\.conf)")
)

type Exporter struct {
	mutex sync.RWMutex

	scrapeFailures prometheus.Counter

	cpuTimeCounter      *prometheus.CounterVec
	virtualMemoryGauge  *prometheus.GaugeVec
	residentMemoryGauge *prometheus.GaugeVec
}

func NewExporter() (*Exporter, error) {
	return &Exporter{
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Number of errors while scraping apache.",
		}),
		cpuTimeCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cpu_time",
				Help:      "fluentd cpu time",
			},
			[]string{"conf_name"},
		),
		virtualMemoryGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "virtual_memory_usage",
				Help:      "fluentd virtual memory usage",
			},
			[]string{"conf_name"},
		),
		residentMemoryGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "resident_memory_usage",
				Help:      "fluentd resident memory usage",
			},
			[]string{"conf_name"},
		),
	}, nil
}

// Describe implements the prometheus.Collector interface.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.scrapeFailures.Describe(ch)
	e.cpuTimeCounter.Describe(ch)
	e.virtualMemoryGauge.Describe(ch)
	e.residentMemoryGauge.Describe(ch)
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {

	out, err := exec.Command("ps", "-C", "ruby", "-f").Output()
	if err != nil {
		log.Fatal(err)
		return err
	}

	fluentdConfNames := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "fluentd") == true && strings.Contains(line, ".conf") == true {
			groups := fluentdConfRegexp.FindStringSubmatch(line)
			fluentdConfNames[groups[1]] = struct{}{}
		} else if strings.Contains(line, "td-agent") == true {
			groups := fluentdConfRegexp.FindStringSubmatch(line)
			var key string
			if len(groups) == 0 {
				key = "td-agent"
			} else {
				key = groups[1]
			}
			fluentdConfNames[key] = struct{}{}
		}
	}

	for fluentdConfName := range fluentdConfNames {
		out, err := exec.Command("pgrep", "-n", "-f", fluentdConfName).Output()
		if err != nil {
			log.Fatal(err)
			return err
		}
		targetPid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			log.Fatal(err)
			return err
		}

		name := strings.TrimSpace(strings.Replace(fluentdConfName, ".conf", "", 1))

		procfsPath := procfs.DefaultMountPoint
		fs, err := procfs.NewFS(procfsPath)
		if err != nil {
			log.Fatal(err)
			return err
		}
		proc, err := fs.NewProc(targetPid)
		if err != nil {
			log.Fatal(err)
			return err
		}
		procStat, err := proc.NewStat()
		if err != nil {
			log.Fatal(err)
			return err
		}
		cpuTime := procStat.CPUTime()
		e.cpuTimeCounter.WithLabelValues(name).Set(cpuTime)
		virtualMemory := procStat.VirtualMemory()
		e.virtualMemoryGauge.WithLabelValues(name).Set(float64(virtualMemory))
		residentMemory := procStat.ResidentMemory()
		e.residentMemoryGauge.WithLabelValues(name).Set(float64(residentMemory))
	}
	e.cpuTimeCounter.Collect(ch)
	e.virtualMemoryGauge.Collect(ch)
	e.residentMemoryGauge.Collect(ch)

	return nil
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()
	if err := e.collect(ch); err != nil {
		log.Infof("Error getting process info: %s", err)
		e.scrapeFailures.Inc()
		e.scrapeFailures.Collect(ch)
	}
	return
}

func main() {
	flag.Parse()

	exporter, err := NewExporter()
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(exporter)

	log.Printf("Starting Server: %s", *listenAddress)
	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		<head><title>fluentd Exporter</title></head>
		<body>
		<h1>fluentd Exporter</h1>
		<p><a href="` + *metricsPath + `">Metrics</a></p>
		</body>
		</html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))

}
