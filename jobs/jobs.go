package jobs

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/eqlabs/flow-wallet-api/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkerPool struct {
	log     *log.Logger
	wg      *sync.WaitGroup
	workers []*Worker
	store   Store
}

type Worker struct {
	pool    *WorkerPool
	jobChan *chan *Job
}

type Job struct {
	ID        uuid.UUID              `json:"jobId" gorm:"primary_key;type:uuid;"`
	Do        func() (string, error) `json:"-" gorm:"-"`
	Status    Status                 `json:"status"`
	Error     string                 `json:"-"`
	Result    string                 `json:"result"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
	DeletedAt gorm.DeletedAt         `json:"-" gorm:"index"`
}

func (j *Job) BeforeCreate(tx *gorm.DB) (err error) {
	j.ID = uuid.New()
	return
}

func (j *Job) Wait(wait bool) error {
	if wait {
		// Wait for the job to have finished
		for j.Status == Accepted {
			time.Sleep(10 * time.Millisecond)
		}
		if j.Status == Error {
			return fmt.Errorf(j.Error)
		}
	}
	return nil
}

func NewWorkerPool(l *log.Logger, db Store) *WorkerPool {
	return &WorkerPool{l, &sync.WaitGroup{}, []*Worker{}, db}
}

func (p *WorkerPool) AddWorker(capacity uint) {
	if len(p.workers) > 0 {
		panic("multiple workers not supported yet")
	}
	p.wg.Add(1)
	jobChan := make(chan *Job, capacity)
	worker := &Worker{p, &jobChan}
	p.workers = append(p.workers, worker)
	go worker.start()
}

func (p *WorkerPool) AddJob(do func() (string, error)) (*Job, error) {
	job := &Job{Do: do, Status: Init}
	if err := p.store.InsertJob(job); err != nil {
		return job, err
	}

	worker, err := p.AvailableWorker()
	if err != nil {
		job.Status = NoAvailableWorkers
		if err := p.store.UpdateJob(job); err != nil {
			p.log.Println("WARNING: Could not update DB entry for Job", job.ID)
		}
		return job, &errors.JobQueueFull{Err: fmt.Errorf(job.Status.String())}
	}

	if !worker.tryEnqueue(job) {
		job.Status = QueueFull
		if err := p.store.UpdateJob(job); err != nil {
			p.log.Println("WARNING: Could not update DB entry for Job", job.ID)
		}
		return job, &errors.JobQueueFull{Err: fmt.Errorf(job.Status.String())}
	}

	job.Status = Accepted
	if err := p.store.UpdateJob(job); err != nil {
		p.log.Println("WARNING: Could not update DB entry for Job", job.ID)
	}

	return job, nil
}

func (p *WorkerPool) AvailableWorker() (*Worker, error) {
	// TODO: support multiple workers, use load balancing
	if len(p.workers) < 1 {
		return nil, fmt.Errorf("no available workers")
	}
	return p.workers[0], nil
}

func (p *WorkerPool) Stop() {
	for _, w := range p.workers {
		close(*w.jobChan)
	}
	p.wg.Wait()
}

func (w *Worker) start() {
	defer w.pool.wg.Done()
	for job := range *w.jobChan {
		if job == nil {
			return
		}
		w.process(job)
	}
}

func (w *Worker) tryEnqueue(job *Job) bool {
	select {
	case *w.jobChan <- job:
		return true
	default:
		return false
	}
}

func (w *Worker) process(job *Job) {
	result, err := job.Do()
	job.Result = result
	if err != nil {
		if w.pool.log != nil {
			w.pool.log.Printf("[Job %s] Error while processing job: %s\n", job.ID, err)
		}
		job.Status = Error
		job.Error = err.Error()
		if err := w.pool.store.UpdateJob(job); err != nil {
			w.pool.log.Println("WARNING: Could not update DB entry for Job", job.ID)
		}
		return
	}
	job.Status = Complete
	if err := w.pool.store.UpdateJob(job); err != nil {
		w.pool.log.Println("WARNING: Could not update DB entry for Job", job.ID)
	}
}
