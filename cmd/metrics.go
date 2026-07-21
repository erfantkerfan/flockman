package cmd

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricsOnce     sync.Once
	metricsRegistry *prometheus.Registry

	httpRequestsTotal          *prometheus.CounterVec
	httpRequestDuration        *prometheus.HistogramVec
	tokenLookupsTotal          *prometheus.CounterVec
	serviceStatusRequestsTotal *prometheus.CounterVec
	serviceUpdateRequestsTotal *prometheus.CounterVec
	serviceUpdateDuration      prometheus.Histogram
	dockerAPIRequestsTotal     *prometheus.CounterVec
	dockerAPIDuration          *prometheus.HistogramVec
	registeredServicesDesc     *prometheus.Desc
)

func initMetrics() {
	metricsOnce.Do(func() {
		metricsRegistry = prometheus.NewRegistry()
		metricsRegistry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		)

		buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "flockman_build_info",
			Help: "Flockman build information.",
		}, []string{"version"})
		buildInfo.WithLabelValues(version).Set(1)
		metricsRegistry.MustRegister(buildInfo)

		httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flockman_http_requests_total",
			Help: "Total number of HTTP API requests.",
		}, []string{"method", "path", "code"})
		httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "flockman_http_request_duration_seconds",
			Help:    "Duration of HTTP API requests in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"})

		tokenLookupsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flockman_token_lookups_total",
			Help: "Total number of service token lookups.",
		}, []string{"result"})

		serviceStatusRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flockman_service_status_requests_total",
			Help: "Total number of service status requests by result.",
		}, []string{"result"})
		serviceUpdateRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flockman_service_update_requests_total",
			Help: "Total number of service update requests by result.",
		}, []string{"result", "start_first"})
		serviceUpdateDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "flockman_service_update_duration_seconds",
			Help:    "Duration of service update requests in seconds.",
			Buckets: prometheus.DefBuckets,
		})

		dockerAPIRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flockman_docker_api_requests_total",
			Help: "Total number of Docker API calls by operation and result.",
		}, []string{"operation", "result"})
		dockerAPIDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "flockman_docker_api_duration_seconds",
			Help:    "Duration of Docker API calls in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation"})

		registeredServicesDesc = prometheus.NewDesc(
			"flockman_registered_services",
			"Number of services registered in the local database.",
			nil,
			nil,
		)

		metricsRegistry.MustRegister(
			httpRequestsTotal,
			httpRequestDuration,
			tokenLookupsTotal,
			serviceStatusRequestsTotal,
			serviceUpdateRequestsTotal,
			serviceUpdateDuration,
			dockerAPIRequestsTotal,
			dockerAPIDuration,
			registeredServicesCollector{},
		)
	})
}

type registeredServicesCollector struct{}

func (registeredServicesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- registeredServicesDesc
}

func (registeredServicesCollector) Collect(ch chan<- prometheus.Metric) {
	var count int64
	if db != nil {
		_ = db.Model(&Service{}).Count(&count).Error
	}
	ch <- prometheus.MustNewConstMetric(registeredServicesDesc, prometheus.GaugeValue, float64(count))
}

func validateMetricsPort(apiPort, metricsPort string) error {
	if metricsPort == "" {
		return nil
	}
	if metricsPort == apiPort {
		return fmt.Errorf("metrics port must differ from API port (%s)", apiPort)
	}
	return nil
}

func newMetricsServer(addr string) *http.Server {
	initMetrics()
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{}))
	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}

func metricsMiddleware() gin.HandlerFunc {
	initMetrics()
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "other"
		}
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, strconv.Itoa(c.Writer.Status())).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

func observeDockerAPI(operation string, started time.Time, err error) {
	initMetrics()
	result := "ok"
	if err != nil {
		result = "error"
	}
	dockerAPIRequestsTotal.WithLabelValues(operation, result).Inc()
	dockerAPIDuration.WithLabelValues(operation).Observe(time.Since(started).Seconds())
}

func recordTokenLookup(result string) {
	initMetrics()
	tokenLookupsTotal.WithLabelValues(result).Inc()
}

func recordServiceStatus(result string) {
	initMetrics()
	serviceStatusRequestsTotal.WithLabelValues(result).Inc()
}

func recordServiceUpdate(result string, startFirst bool) {
	initMetrics()
	startFirstLabel := "false"
	if startFirst {
		startFirstLabel = "true"
	}
	serviceUpdateRequestsTotal.WithLabelValues(result, startFirstLabel).Inc()
}

func observeServiceUpdateDuration(started time.Time) {
	initMetrics()
	serviceUpdateDuration.Observe(time.Since(started).Seconds())
}
