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
)

const (
	namespace = "fluentd"
)

var (
	listenAddress = flag.String("web.listen-address", ":9224", "Address on which to expose metrics and web interface.")
	metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
)

type Exporter struct {
	mutex sync.RWMutex

	scrapeFailures prometheus.Counter

	cpuGauge *prometheus.GaugeVec
	vszGauge *prometheus.GaugeVec
	rssGauge *prometheus.GaugeVec
}

func NewExporter() (*Exporter, error) {
	return &Exporter{
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Number of errors while scraping apache.",
		}),
		cpuGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "cpu_usage",
				Help:      "fluentd cpu usage",
			},
			[]string{"conf_name"},
		),
		vszGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "vsz_usage",
				Help:      "fluentd vsz usage",
			},
			[]string{"conf_name"},
		),
		rssGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "rss_usage",
				Help:      "fluentd rss usage",
			},
			[]string{"conf_name"},
		),
	}, nil
}

// Describe implements the prometheus.Collector interface.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.scrapeFailures.Describe(ch)
	e.cpuGauge.Describe(ch)
	e.vszGauge.Describe(ch)
	e.rssGauge.Describe(ch)
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {

	out, psErr := exec.Command("ps", "-C", "ruby", "-f").Output()
	if psErr != nil {
		log.Fatal(psErr)
	}

	fluentdConfNames := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "fluentd") == true && strings.Contains(line, ".conf") == true {
			rep := regexp.MustCompile("\\W([\\w\\.]+\\.conf)")
			groups := rep.FindStringSubmatch(line)
			fluentdConfNames[groups[1]] = struct{}{}
		}
	}

	for fluentdConfName := range fluentdConfNames {
		out, pgrepErr := exec.Command("pgrep", "-n", "-f", fluentdConfName).Output()
		if pgrepErr != nil {
			log.Fatal(pgrepErr)
		}
		targetPid := strings.TrimSpace(string(out))
		name := strings.TrimSpace(strings.Replace(fluentdConfName, ".conf", "", 1))
		out, pidstatErr := exec.Command("pidstat", "-h", "-u", "-r", "-p", targetPid, "5", "1").Output()
		if pidstatErr != nil {
			log.Fatal(pidstatErr)
		}

		for i, line := range strings.Split(string(out), "\n") {
			if i == 3 {
				parts := strings.Fields(line)
				cpu, err := strconv.ParseFloat(parts[5], 64)
				if err != nil {
					log.Fatal(err)
				}
				e.cpuGauge.WithLabelValues(name).Set(cpu)

				vsz, err := strconv.ParseFloat(parts[9], 64)
				if err != nil {
					log.Fatal(err)
				}
				e.vszGauge.WithLabelValues(name).Set(vsz)

				rss, err := strconv.ParseFloat(parts[10], 64)
				if err != nil {
					log.Fatal(err)
				}
				e.rssGauge.WithLabelValues(name).Set(rss)
			}
		}
	}
	e.cpuGauge.Collect(ch)
	e.vszGauge.Collect(ch)
	e.rssGauge.Collect(ch)

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
