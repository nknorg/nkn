// This package is for CONcurrent SEQUENTIAL (consequential) jobs that can run
// concurrently, but can only be finished (verified) sequentially.

package consequential

import (
	"fmt"
	"sync"
	"time"
)

// RunJob runs a job with job id, returns result and whether success
type RunJob func(workerID, jobID uint32) (result interface{}, success bool)

// FinishJob finish a job with job id, returns whether success
type FinishJob func(jobID uint32, result interface{}) (success bool)

// Config is the ConSequential config.
type Config struct {
	StartJobID          uint32
	EndJobID            uint32
	JobBufSize          uint32
	WorkerPoolSize      uint32
	MaxWorkerFails      uint32
	WorkerStartInterval time.Duration
	RunJob              RunJob
	FinishJob           FinishJob
}

// ConSequential is short for CONcurrent SEQUENTIAL
type ConSequential struct {
	*Config
	unstartedJobChan chan uint32
	failedJobChan    chan uint32
	stopChans        []chan struct{}

	sync.RWMutex
	jobResultBuf      []interface{}
	ringBufStartJobID uint32
	ringBufStartIdx   uint32
}

// NewConSequential creates a new ConSequential struct
func NewConSequential(config *Config) (*ConSequential, error) {
	cs := &ConSequential{
		Config:            config,
		unstartedJobChan:  make(chan uint32, config.JobBufSize),
		failedJobChan:     make(chan uint32, config.JobBufSize),
		stopChans:         make([]chan struct{}, config.WorkerPoolSize, config.WorkerPoolSize),
		jobResultBuf:      make([]interface{}, config.JobBufSize, config.JobBufSize),
		ringBufStartJobID: config.StartJobID,
	}
	for i := uint32(0); i < config.WorkerPoolSize; i++ {
		cs.stopChans[i] = make(chan struct{}, 1)
	}
	return cs, nil
}

func (cs *ConSequential) isJobIDInRange(jobID uint32) bool {
	if cs.StartJobID <= cs.EndJobID {
		return jobID >= cs.StartJobID && jobID <= cs.EndJobID
	}
	return jobID <= cs.StartJobID && jobID >= cs.EndJobID
}

func (cs *ConSequential) initJobChan() {
	var i uint32
	for i = 0; i < cs.JobBufSize; i++ {
		if cs.StartJobID <= cs.EndJobID {
			if cs.isJobIDInRange(cs.StartJobID + i) {
				cs.unstartedJobChan <- cs.StartJobID + i
			}
		} else {
			if cs.isJobIDInRange(cs.StartJobID - i) {
				cs.unstartedJobChan <- cs.StartJobID - i
			}
		}
	}
}

// shiftRingBuf shifts the ring buffer by one job id
func (cs *ConSequential) shiftRingBuf() {
	if !cs.isJobIDInRange(cs.ringBufStartJobID) {
		return
	}

	cs.jobResultBuf[cs.ringBufStartIdx] = nil
	cs.ringBufStartIdx = (cs.ringBufStartIdx + 1) % cs.JobBufSize

	if cs.StartJobID < cs.EndJobID {
		cs.ringBufStartJobID++
	} else {
		cs.ringBufStartJobID--
	}

	var ringBufEndJobID uint32
	if cs.StartJobID < cs.EndJobID {
		ringBufEndJobID = cs.ringBufStartJobID + cs.JobBufSize - 1
	} else {
		if cs.ringBufStartJobID+1 > cs.JobBufSize {
			ringBufEndJobID = cs.ringBufStartJobID + 1 - cs.JobBufSize
		} else {
			ringBufEndJobID = 0
		}
	}

	if cs.isJobIDInRange(ringBufEndJobID) {
		cs.unstartedJobChan <- ringBufEndJobID
	}

	if !cs.isJobIDInRange(cs.ringBufStartJobID) {
		for _, c := range cs.stopChans {
			select {
			case c <- struct{}{}:
			default:
			}
		}
	}
}

// Start starts workers concurrently
func (cs *ConSequential) Start() error {
	cs.initJobChan()

	var wg sync.WaitGroup
	var workerID uint32
	for workerID = 0; workerID < cs.WorkerPoolSize; workerID++ {
		wg.Add(1)
		go func(workerID uint32) {
			defer wg.Done()
			if cs.WorkerStartInterval > 0 {
				time.Sleep(time.Duration(workerID) * cs.WorkerStartInterval)
			}
			cs.startWorker(workerID)
		}(workerID)
	}
	wg.Wait()

	cs.RLock()
	defer cs.RUnlock()
	if cs.isJobIDInRange(cs.ringBufStartJobID) {
		return fmt.Errorf("all workers failed")
	}

	return nil
}

// startWorker starts a worker, and returns if any job fails to run or all jobs
// have been finished
func (cs *ConSequential) startWorker(workerID uint32) {
	var jobID, failCount uint32

	for {
		select {
		case jobID = <-cs.failedJobChan:
		case jobID = <-cs.unstartedJobChan:
		case <-cs.stopChans[workerID]:
			return
		}

		if !cs.tryJob(workerID, jobID) {
			failCount++
		}

		if failCount > cs.MaxWorkerFails {
			return
		}
	}
}

// tryJob tries to run a job and returns whether success
func (cs *ConSequential) tryJob(workerID, jobID uint32) bool {
	result, success := cs.RunJob(workerID, jobID)
	if !success {
		cs.failedJobChan <- jobID
		return false
	}

	cs.Lock()
	defer cs.Unlock()

	var idxOffset uint32
	if jobID > cs.ringBufStartJobID {
		idxOffset = jobID - cs.ringBufStartJobID
	} else {
		idxOffset = cs.ringBufStartJobID - jobID
	}

	cs.jobResultBuf[(cs.ringBufStartIdx+idxOffset)%cs.JobBufSize] = result

	if idxOffset > 0 {
		return true
	}

	numFinished := cs.tryFinishJobs()
	return numFinished > 0
}

// tryFinishJobs tries to finish as many jobs as possible, starting from job id
// ringBufStartJobID. Returns number of successfully finished jobs.
func (cs *ConSequential) tryFinishJobs() uint32 {
	var jobID, ringBufStartIdx uint32
	var result interface{}
	var success bool
	var numFinished uint32

	for {
		jobID = cs.ringBufStartJobID
		ringBufStartIdx = cs.ringBufStartIdx
		result = cs.jobResultBuf[ringBufStartIdx]

		if result == nil {
			break
		}

		success = cs.FinishJob(jobID, result)
		if success {
			cs.shiftRingBuf()
			numFinished++
		} else {
			cs.jobResultBuf[ringBufStartIdx] = nil
			cs.failedJobChan <- jobID
		}
	}

	return numFinished
}
