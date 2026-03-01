package observability

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Logger interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
}

type DefaultLogger struct {
	mu     sync.RWMutex
	level  LogLevel
	output Output
	format Format
}

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

type Output int

const (
	OutputStdout Output = iota
	OutputStderr
	OutputFile
)

type Format int

const (
	FormatText Format = iota
	FormatJSON
)

func NewDefaultLogger(level LogLevel) *DefaultLogger {
	return &DefaultLogger{
		level:  level,
		output: OutputStdout,
		format: FormatText,
	}
}

func (l *DefaultLogger) Debug(args ...interface{}) {
	if l.level <= LevelDebug {
		l.log(LevelDebug, args...)
	}
}

func (l *DefaultLogger) Info(args ...interface{}) {
	if l.level <= LevelInfo {
		l.log(LevelInfo, args...)
	}
}

func (l *DefaultLogger) Warn(args ...interface{}) {
	if l.level <= LevelWarn {
		l.log(LevelWarn, args...)
	}
}

func (l *DefaultLogger) Error(args ...interface{}) {
	if l.level <= LevelError {
		l.log(LevelError, args...)
	}
}

func (l *DefaultLogger) Fatal(args ...interface{}) {
	l.log(LevelFatal, args...)
	panic("fatal")
}

func (l *DefaultLogger) log(level LogLevel, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	timestamp := time.Now().Format(time.RFC3339)
	levelStr := []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[level]

	output := l.output
	if output == OutputStdout && level >= LevelError {
		output = OutputStderr
	}

	message := fmt.Sprint(args...)

	switch l.format {
	case FormatJSON:
		fmt.Printf(`{"timestamp":"%s","level":"%s","message":"%s"}`+"\n", timestamp, levelStr, message)
	default:
		fmt.Printf("[%s] %s: %s\n", timestamp, levelStr, message)
	}

	_ = output
}

func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *DefaultLogger) SetFormat(format Format) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.format = format
}

type Metrics struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

type Counter struct {
	value uint64
	mu    sync.Mutex
}

type Gauge struct {
	value float64
	mu    sync.Mutex
}

type Histogram struct {
	values []float64
	mu     sync.Mutex
}

func NewMetrics() *Metrics {
	return &Metrics{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

func (m *Metrics) Counter(name string) *Counter {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, exists := m.counters[name]; exists {
		return c
	}

	c := &Counter{}
	m.counters[name] = c
	return c
}

func (m *Metrics) Gauge(name string) *Gauge {
	m.mu.Lock()
	defer m.mu.Unlock()

	if g, exists := m.gauges[name]; exists {
		return g
	}

	g := &Gauge{}
	m.gauges[name] = g
	return g
}

func (m *Metrics) Histogram(name string) *Histogram {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, exists := m.histograms[name]; exists {
		return h
	}

	h := &Histogram{}
	m.histograms[name] = h
	return h
}

func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

func (c *Counter) Add(n uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += n
}

func (c *Counter) Value() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = value
}

func (g *Gauge) Inc() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
}

func (g *Gauge) Dec() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value--
}

func (g *Gauge) Add(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value += value
}

func (g *Gauge) GetValue() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.value
}

func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.values = append(h.values, value)
}

func (h *Histogram) Values() []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.values
}

type Tracer interface {
	StartSpan(name string) Span
	WithContext(ctx context.Context) context.Context
}

type Span interface {
	End()
	SetAttribute(key string, value interface{})
	SetError(err error)
}

type DefaultTracer struct {
	mu     sync.RWMutex
	spans  map[string]*DefaultSpan
	active *DefaultSpan
}

type DefaultSpan struct {
	name       string
	startTime  time.Time
	endTime    *time.Time
	attributes map[string]interface{}
	err        error
	mu         sync.RWMutex
}

func NewDefaultTracer() *DefaultTracer {
	return &DefaultTracer{
		spans: make(map[string]*DefaultSpan),
	}
}

func (t *DefaultTracer) StartSpan(name string) Span {
	t.mu.Lock()
	defer t.mu.Unlock()

	span := &DefaultSpan{
		name:       name,
		startTime:  time.Now(),
		attributes: make(map[string]interface{}),
	}

	t.spans[name] = span
	t.active = span

	return span
}

func (t *DefaultTracer) WithContext(ctx context.Context) context.Context {
	return ctx
}

func (s *DefaultSpan) End() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endTime = &now
}

func (s *DefaultSpan) SetAttribute(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attributes[key] = value
}

func (s *DefaultSpan) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

type MetricsExporter interface {
	Export(ctx context.Context, metrics *Metrics) error
}

type LogExporter struct {
	logger Logger
}

func NewLogExporter(logger Logger) *LogExporter {
	return &LogExporter{logger: logger}
}

func (e *LogExporter) Export(ctx context.Context, metrics *Metrics) error {
	e.logger.Info("Exporting metrics")
	return nil
}
