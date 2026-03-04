package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Daemon struct {
	name     string
	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}
	workers  map[string]*Worker
	manager  *WorkerManager
	config   *DaemonConfig
}

type DaemonConfig struct {
	Name       string
	Workers    int
	MaxRetries int
	Timeout    time.Duration
}

type Worker struct {
	ID        string
	Name      string
	Job       Job
	Status    WorkerStatus
	StartedAt time.Time
	EndedAt   *time.Time
	Error     error
	mu        sync.RWMutex
}

type WorkerStatus string

const (
	WorkerStatusPending   WorkerStatus = "pending"
	WorkerStatusRunning   WorkerStatus = "running"
	WorkerStatusCompleted WorkerStatus = "completed"
	WorkerStatusFailed    WorkerStatus = "failed"
	WorkerStatusStopped   WorkerStatus = "stopped"
)

type Job interface {
	Run(ctx context.Context) error
}

type WorkerManager struct {
	mu       sync.RWMutex
	workers  map[string]*Worker
	jobQueue chan Job
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewWorkerManager(workers int) *WorkerManager {
	return &WorkerManager{
		workers:  make(map[string]*Worker),
		jobQueue: make(chan Job, workers*2),
		stopChan: make(chan struct{}),
	}
}

func (m *WorkerManager) Start(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go m.worker(ctx, i)
	}
}

func (m *WorkerManager) worker(ctx context.Context, id int) {
	defer m.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case job := <-m.jobQueue:
			if err := job.Run(ctx); err != nil {
				fmt.Printf("Worker %d: job failed: %v\n", id, err)
			}
		}
	}
}

func (m *WorkerManager) Submit(job Job) {
	select {
	case m.jobQueue <- job:
	default:
		fmt.Println("Job queue full")
	}
}

func (m *WorkerManager) Stop() {
	close(m.stopChan)
	m.wg.Wait()
}

func NewDaemon(name string) *Daemon {
	return &Daemon{
		name:     name,
		stopChan: make(chan struct{}),
		workers:  make(map[string]*Worker),
	}
}

func (d *Daemon) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("daemon already running")
	}
	d.running = true
	d.mu.Unlock()

	go d.run(ctx)

	return nil
}

func (d *Daemon) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return fmt.Errorf("daemon not running")
	}

	close(d.stopChan)
	d.running = false

	return nil
}

func (d *Daemon) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

func (d *Daemon) run(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		d.Stop(context.Background())
	case <-d.stopChan:
		d.Stop(context.Background())
	case <-sigChan:
		fmt.Println("Received shutdown signal")
		d.Stop(context.Background())
	}
}

func (d *Daemon) AddWorker(worker *Worker) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.workers[worker.ID] = worker
}

func (d *Daemon) RemoveWorker(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.workers, id)
}

func (d *Daemon) GetWorker(id string) (*Worker, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	w, exists := d.workers[id]
	return w, exists
}

func (d *Daemon) ListWorkers() []*Worker {
	d.mu.RLock()
	defer d.mu.RUnlock()

	workers := make([]*Worker, 0, len(d.workers))
	for _, w := range d.workers {
		workers = append(workers, w)
	}

	return workers
}

func (w *Worker) GetStatus() WorkerStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Status
}

func (w *Worker) StartJob() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Status = WorkerStatusRunning
	w.StartedAt = time.Now()
}

func (w *Worker) EndJob(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	w.EndedAt = &now
	w.Error = err
	if err != nil {
		w.Status = WorkerStatusFailed
	} else {
		w.Status = WorkerStatusCompleted
	}
}

func (w *Worker) StopJob() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Status = WorkerStatusStopped
	now := time.Now()
	w.EndedAt = &now
}

type Scheduler struct {
	mu       sync.RWMutex
	tasks    map[string]*ScheduledTask
	stopChan chan struct{}
	wg       sync.WaitGroup
}

type ScheduledTask struct {
	ID       string
	Name     string
	Schedule string
	Job      Job
	LastRun  *time.Time
	NextRun  *time.Time
	Enabled  bool
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		tasks:    make(map[string]*ScheduledTask),
		stopChan: make(chan struct{}),
	}
}

func (s *Scheduler) AddTask(task *ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task already exists: %s", task.ID)
	}

	s.tasks[task.ID] = task
	return nil
}

func (s *Scheduler) RemoveTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	delete(s.tasks, id)
	return nil
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.RLock()
	tasks := s.tasks
	s.mu.RUnlock()

	for _, task := range tasks {
		s.wg.Add(1)
		go s.runTask(ctx, task)
	}
}

func (s *Scheduler) runTask(ctx context.Context, task *ScheduledTask) {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			if task.Enabled {
				task.LastRun = new(time.Time)
				*task.LastRun = time.Now()
				_ = task.Job.Run(ctx)
			}
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}
