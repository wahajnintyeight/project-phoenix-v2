package service

import (
	"log"
	"sync"

	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/google"
)

type DownloadJob struct {
	ID       string
	VideoID  string
	Format   string
	Quality  string
	BitRate  string
	VideoTitle string
	Status   enum.SSEStreamEnum // "queued", "downloading", "completed", "error"
	Progress int
	FilePath string
	FileSize int64
	session  *google.StreamSession
	mu       sync.Mutex
}
type DownloadQueue struct {
	jobs            map[string]*DownloadJob
	queue           chan *DownloadJob
	activeDownloads map[string]*DownloadJob
	maxConcurrent   int
	mu              sync.RWMutex
	sseService      *SSEService
}

func NewDownloadQueue(maxConcurrent int, sse *SSEService) *DownloadQueue {
	dq := &DownloadQueue{
		jobs:            make(map[string]*DownloadJob),
		queue:           make(chan *DownloadJob, 100),
		activeDownloads: make(map[string]*DownloadJob),
		maxConcurrent:   maxConcurrent,
		sseService:      sse,
	}

	for i := 0; i < maxConcurrent; i++ {
		go dq.worker(i)
	}

	return dq
}

func (dq *DownloadQueue) worker(id int) {
	for job := range dq.queue {
		dq.mu.Lock()
		dq.activeDownloads[job.ID] = job
		dq.mu.Unlock()

		log.Printf("🟢 [WORKER %d] Processing: %s", id, job.ID)

		job.mu.Lock()
		job.Status = enum.DOWNLOADING
		job.mu.Unlock()

		dq.sseService.processVideoDownload(
			job.ID,
			job.VideoID,
			job.Format,
			job.Quality,
			job.BitRate,
			job.VideoTitle,
		)

		dq.mu.Lock()
		delete(dq.activeDownloads, job.ID)
		dq.mu.Unlock()

		log.Printf(" [WORKER %d] Finished: %s (status: %s)", id, job.ID, job.Status)
	}
}

func (dq *DownloadQueue) AddJob(id, videoID, format, quality, bitRate, videoTitle string) {
	job := &DownloadJob{
		ID:       id,
		VideoID:  videoID,
		Format:   format,
		Quality:  quality,
		BitRate:  bitRate,
		VideoTitle: videoTitle,
		Status:   enum.QUEUED,
		Progress: 0,
	}

	dq.mu.Lock()
	dq.jobs[id] = job
	dq.mu.Unlock()

	dq.queue <- job
	log.Printf(" Download queued: %s", id)
	log.Printf("📋 Download queued: %s", id)
}

func (dq *DownloadQueue) GetJob(id string) *DownloadJob {
	dq.mu.RLock()
	defer dq.mu.RUnlock()
	return dq.jobs[id]
}
