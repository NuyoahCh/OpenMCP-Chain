package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// requestKey 用于标识 HTTP 请求的唯一键。
type requestKey struct {
	handler string
	method  string
	code    string
}

// errorKey 用于标识 HTTP 请求错误的唯一键。
type errorKey struct {
	handler string
	method  string
}

// latencyKey 用于标识 HTTP 请求延迟的唯一键。
type latencyKey struct {
	handler string
	method  string
}

// histogram 用于记录 HTTP 请求的延迟分布情况。
type histogram struct {
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

// collector 收集和存储 HTTP 请求的指标数据。
type collector struct {
	mu       sync.Mutex
	requests map[requestKey]uint64
	errors   map[errorKey]uint64
	latency  map[latencyKey]*histogram
}

// 全局 HTTP 指标收集器实例。
var httpCollector = &collector{
	requests: make(map[requestKey]uint64),
	errors:   make(map[errorKey]uint64),
	latency:  make(map[latencyKey]*histogram),
}

// ObserveHTTPRequest 记录一次 HTTP 请求的指标数据。
func ObserveHTTPRequest(handler, method string, status int, duration time.Duration) {
	httpCollector.observe(handler, method, status, duration)
}

// observe 记录一次 HTTP 请求的指标数据。
func (c *collector) observe(handler, method string, status int, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	reqKey := requestKey{handler: handler, method: method, code: strconv.Itoa(status)}
	c.requests[reqKey]++
	if status >= 500 {
		errKey := errorKey{handler: handler, method: method}
		c.errors[errKey]++
	}

	latKey := latencyKey{handler: handler, method: method}
	hist := c.latency[latKey]
	if hist == nil {
		hist = newHistogram()
		c.latency[latKey] = hist
	}
	hist.observe(duration.Seconds())
}

// newHistogram 创建并初始化一个新的直方图实例。
func newHistogram() *histogram {
	buckets := []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	return &histogram{
		buckets: buckets,
		counts:  make([]uint64, len(buckets)),
	}
}

// observe 在直方图中记录一个新的观测值。
func (h *histogram) observe(value float64) {
	h.count++
	h.sum += value
	placed := false
	for idx, bound := range h.buckets {
		if value <= bound {
			for i := idx; i < len(h.counts); i++ {
				h.counts[i]++
			}
			placed = true
			break
		}
	}
	if !placed {
		// Values greater than the last bucket are accounted for in the +Inf bucket via h.count.
	}
}

// Handler 返回一个 HTTP 处理器，用于暴露 Prometheus 格式的指标数据。
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = fmt.Fprint(w, httpCollector.render())
	})
}

// render 生成 Prometheus 格式的指标数据字符串。
func (c *collector) render() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	type requestMetric struct {
		requestKey
		value uint64
	}
	type errorMetric struct {
		errorKey
		value uint64
	}
	type latencyMetric struct {
		latencyKey
		buckets []float64
		counts  []uint64
		sum     float64
		count   uint64
	}

	reqs := make([]requestMetric, 0, len(c.requests))
	for key, value := range c.requests {
		reqs = append(reqs, requestMetric{requestKey: key, value: value})
	}
	errs := make([]errorMetric, 0, len(c.errors))
	for key, value := range c.errors {
		errs = append(errs, errorMetric{errorKey: key, value: value})
	}
	lats := make([]latencyMetric, 0, len(c.latency))
	for key, hist := range c.latency {
		lats = append(lats, latencyMetric{
			latencyKey: key,
			buckets:    append([]float64(nil), hist.buckets...),
			counts:     append([]uint64(nil), hist.counts...),
			sum:        hist.sum,
			count:      hist.count,
		})
	}

	sort.Slice(reqs, func(i, j int) bool {
		if reqs[i].handler == reqs[j].handler {
			if reqs[i].method == reqs[j].method {
				return reqs[i].code < reqs[j].code
			}
			return reqs[i].method < reqs[j].method
		}
		return reqs[i].handler < reqs[j].handler
	})
	sort.Slice(errs, func(i, j int) bool {
		if errs[i].handler == errs[j].handler {
			return errs[i].method < errs[j].method
		}
		return errs[i].handler < errs[j].handler
	})
	sort.Slice(lats, func(i, j int) bool {
		if lats[i].handler == lats[j].handler {
			return lats[i].method < lats[j].method
		}
		return lats[i].handler < lats[j].handler
	})

	var builder strings.Builder
	builder.Grow(1024)

	builder.WriteString("# HELP openmcp_http_requests_total Total number of HTTP requests processed.\n")
	builder.WriteString("# TYPE openmcp_http_requests_total counter\n")
	for _, metric := range reqs {
		builder.WriteString(fmt.Sprintf("openmcp_http_requests_total{handler=\"%s\",method=\"%s\",code=\"%s\"} %d\n",
			escape(metric.handler), escape(metric.method), escape(metric.code), metric.value))
	}

	builder.WriteString("# HELP openmcp_http_request_errors_total Total number of HTTP requests that resulted in a server error.\n")
	builder.WriteString("# TYPE openmcp_http_request_errors_total counter\n")
	for _, metric := range errs {
		builder.WriteString(fmt.Sprintf("openmcp_http_request_errors_total{handler=\"%s\",method=\"%s\"} %d\n",
			escape(metric.handler), escape(metric.method), metric.value))
	}

	builder.WriteString("# HELP openmcp_http_request_duration_seconds HTTP request duration in seconds.\n")
	builder.WriteString("# TYPE openmcp_http_request_duration_seconds histogram\n")
	for _, metric := range lats {
		for idx, bound := range metric.buckets {
			builder.WriteString(fmt.Sprintf("openmcp_http_request_duration_seconds_bucket{handler=\"%s\",method=\"%s\",le=\"%s\"} %d\n",
				escape(metric.handler), escape(metric.method), formatFloat(bound), metric.counts[idx]))
		}
		builder.WriteString(fmt.Sprintf("openmcp_http_request_duration_seconds_bucket{handler=\"%s\",method=\"%s\",le=\"+Inf\"} %d\n",
			escape(metric.handler), escape(metric.method), metric.count))
		builder.WriteString(fmt.Sprintf("openmcp_http_request_duration_seconds_sum{handler=\"%s\",method=\"%s\"} %s\n",
			escape(metric.handler), escape(metric.method), formatFloat(metric.sum)))
		builder.WriteString(fmt.Sprintf("openmcp_http_request_duration_seconds_count{handler=\"%s\",method=\"%s\"} %d\n",
			escape(metric.handler), escape(metric.method), metric.count))
	}

	return builder.String()
}

// escape 转义字符串中的特殊字符，确保符合 Prometheus 格式要求。
func escape(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

// formatFloat 将浮点数格式化为字符串。
func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// StartServer 启动一个 HTTP 服务器以暴露指标数据，直到上下文取消或出现错误。
func StartServer(ctx context.Context, addr string) error {
	if addr == "" {
		return errors.New("metrics address is empty")
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", Handler())

	srv := &http.Server{Addr: addr, Handler: mux}
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return ctx.Err()
	case err, ok := <-errCh:
		if !ok {
			return nil
		}
		return err
	}
}
