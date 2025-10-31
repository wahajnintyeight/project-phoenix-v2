package enum

type SSEStreamEnum string

const (
	QUEUED      = "queued"
	DOWNLOADING = "downloading"
	PROCESSING  = "processing"
	COMPLETED   = "completed"
)
