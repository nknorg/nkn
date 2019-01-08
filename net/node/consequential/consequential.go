// This package is for CONcurrent SEQUENTIAL (consequential) jobs that can run
// concurrently, but can only be finished (verified) sequentially.

package consequential

import (
	"sync"
)

// RunJob runs a job with job id, returns result and whether success
type RunJob func(workerID, jobID uint32) (result interface{}, success bool)

// FinishJob finish a job with job id, returns whether success
type FinishJob func(jobID uint32, result interface{}) (success bool)

// ConSequential is short for CONcurrent SEQUENTIAL
type ConSequential struct {
	startJobID       uint32
	endJobID         uint32
	jobBufSize       uint32
	workerPoolSize   uint32
	runJob           RunJob
	finishJob        FinishJob
	unstartedJobChan chan uint32
	failedJobChan    chan uint32

	sync.RWMutex
	jobResultBuf      []interface{}
	ringBufStartJobID uint32
	ringBufStartIdx   uint32
}

// NewConSequential creates a new ConSequential struct
func NewConSequential(startJobID, endJobID, jobBufSize, workerPoolSize uint32, runJob RunJob, finishJob FinishJob) (*ConSequential, error) {
	cs := &ConSequential{
		startJobID:        startJobID,
		endJobID:          endJobID,
		jobBufSize:        jobBufSize,
		workerPoolSize:    workerPoolSize,
		runJob:            runJob,
		finishJob:         finishJob,
		unstartedJobChan:  make(chan uint32, jobBufSize),
		failedJobChan:     make(chan uint32, jobBufSize),
		jobResultBuf:      make([]interface{}, jobBufSize, jobBufSize),
		ringBufStartJobID: startJobID,
	}
	return cs, nil
}

func (cs *ConSequential) isJobIDInRange(jobID uint32) bool {
	if cs.startJobID <= cs.endJobID {
		return jobID >= cs.startJobID && jobID <= cs.endJobID
	}
	return jobID <= cs.startJobID && jobID >= cs.endJobID
}

func (cs *ConSequential) initJobChan() {
	var i uint32
	for i = 0; i < cs.jobBufSize; i++ {
		if cs.startJobID <= cs.endJobID {
			if cs.isJobIDInRange(cs.startJobID + i) {
				cs.unstartedJobChan <- cs.startJobID + i
			}
		} else {
			if cs.isJobIDInRange(cs.startJobID - i) {
				cs.unstartedJobChan <- cs.startJobID - i
			}
		}
	}
}

// shiftRingBuf shifts the ring buffer by one job id
func (cs *ConSequential) shiftRingBuf() {
	cs.jobResultBuf[cs.ringBufStartIdx] = nil
	cs.ringBufStartIdx = (cs.ringBufStartIdx + 1) % cs.jobBufSize

	if cs.startJobID < cs.endJobID {
		cs.ringBufStartJobID++
	} else {
		cs.ringBufStartJobID--
	}

	var ringBufEndJobID uint32
	if cs.startJobID < cs.endJobID {
		ringBufEndJobID = cs.ringBufStartJobID + cs.jobBufSize - 1
	} else {
		if cs.ringBufStartJobID+1 > cs.jobBufSize {
			ringBufEndJobID = cs.ringBufStartJobID + 1 - cs.jobBufSize
		} else {
			ringBufEndJobID = 0
		}
	}

	if cs.isJobIDInRange(ringBufEndJobID) {
		cs.unstartedJobChan <- ringBufEndJobID
	}
}

// Start starts workers concurrently
func (cs *ConSequential) Start() {
	cs.initJobChan()

	var wg sync.WaitGroup
	var workerID uint32
	for workerID = 0; workerID < cs.workerPoolSize; workerID++ {
		wg.Add(1)
		go func(workerID uint32) {
			defer wg.Done()
			cs.startWorker(workerID)
		}(workerID)
	}
	wg.Wait()
}

// startWorker starts a worker, and returns if any job fails to run or all jobs
// have been finished
func (cs *ConSequential) startWorker(workerID uint32) {
	var jobID uint32
	var success bool

	for {
		select {
		case jobID = <-cs.failedJobChan:
			success = cs.tryJob(workerID, jobID)
			if !success {
				return
			}
		default:
		}

		select {
		case jobID = <-cs.unstartedJobChan:
			success = cs.tryJob(workerID, jobID)
			if !success {
				return
			}
		default:
		}

		cs.RLock()
		if !cs.isJobIDInRange(cs.ringBufStartJobID) {
			cs.RUnlock()
			return
		}
		cs.RUnlock()
	}
}

// tryJob tries to run a job and returns whether success
func (cs *ConSequential) tryJob(workerID, jobID uint32) bool {
	result, success := cs.runJob(workerID, jobID)
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

	cs.jobResultBuf[(cs.ringBufStartIdx+idxOffset)%cs.jobBufSize] = result

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

		success = cs.finishJob(jobID, result)
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
