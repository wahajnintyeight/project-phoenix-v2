package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"project-phoenix/v2/internal/aws"
	"project-phoenix/v2/internal/controllers"
	"project-phoenix/v2/internal/enum"
	"project-phoenix/v2/internal/google"
	"project-phoenix/v2/internal/model"
	internal "project-phoenix/v2/internal/service-configs"
	"project-phoenix/v2/pkg/handler"
	"project-phoenix/v2/pkg/helper"
	"project-phoenix/v2/pkg/service"

	"github.com/go-micro/plugins/v4/broker/rabbitmq"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"
)

type SSEService struct {
	service            micro.Service
	router             *mux.Router
	server             *http.Server
	serviceConfig      internal.ServiceConfig
	subscribedServices []internal.SubscribedServices
	brokerObj          microBroker.Broker
	sseHandler         *handler.SSERequestHandler
	downloadQueue      *DownloadQueue
	s3Service          *aws.S3Service
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	re := regexp.MustCompile(`[^a-zA-Z0-9-_ ]+`)
	name = re.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	if len(name) == 0 {
		name = "video"
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}

var once sync.Once

const (
	MaxRetries  = 6
	RetryDelay  = 2 * time.Second
	serviceName = "sse-service"
)

func (sse *SSEService) GetSubscribedTopics() []internal.SubscribedServices {
	serviceConfig, e := internal.ReturnServiceConfig(serviceName)
	if e != nil {
		log.Println("Unable to read service config", e)
		return nil
	}
	sse.subscribedServices = serviceConfig.(*internal.ServiceConfig).SubscribedServices
	log.Println("SSE Service Subscribed Services: ", sse.subscribedServices)
	return sse.subscribedServices
}

func (sse *SSEService) InitializeService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	once.Do(func() {
		service := serviceObj
		sse.service = service
		sse.router = mux.NewRouter()
		sse.brokerObj = service.Options().Broker
		godotenv.Load()
		servicePath := "sse-service"
		serviceConfig, _ := internal.ReturnServiceConfig(servicePath)
		sse.serviceConfig = serviceConfig.(internal.ServiceConfig)

		// Initialize the SSE handler
		sse.sseHandler = handler.NewSSERequestHandler()
		go sse.sseHandler.Run()

		// Initialize S3 service
		s3Svc, _, err := aws.NewS3ServiceFromEnv()
		if err != nil {
			log.Printf("Warning: S3 service initialization failed: %v. File uploads will not work.", err)
		} else {
			sse.s3Service = s3Svc
			log.Println("S3 service initialized successfully")
		}
	})

	return sse
}

func (sse *SSEService) ListenSubscribedTopics(broker microBroker.Event) error {
	// ls.brokerObj.Subscribe()
	// broker
	log.Println("Broker Event: ", broker)
	log.Println("Broker Event: ", broker.Message().Header)
	return nil
}

func (ls *SSEService) SubscribeTopics() {
	ls.InitServiceConfig()
	for _, service := range ls.serviceConfig.SubscribedServices {
		log.Println("Service", service)
		for _, topic := range service.SubscribedTopics {
			log.Println("Preparing to subscribe to service: ", service.Name, " | Topic: ", topic.TopicName, " | Queue: ", service.Queue, " | Handler: ", topic.TopicHandler, " | MaxRetries: ", MaxRetries)
			if err := ls.attemptSubscribe(service.Queue, topic); err != nil {
				log.Printf("Subscription failed for topic %s: %v", topic.TopicName, err)
			}
		}
	}
}

// attemptSubscribe tries to subscribe to a topic with retries until successful or max retries reached.
func (sse *SSEService) attemptSubscribe(queueName string, topic internal.SubscribedTopicsMap) error {
	log.Println("Max Retries:", MaxRetries)
	for i := 0; i <= MaxRetries; i++ {
		log.Println("Attempting to subscribe to", topic, " for Queue", queueName)
		if err := sse.subscribeToTopic(queueName, topic); err != nil {
			if err.Error() == "not connected" && i < MaxRetries {
				log.Printf("Broker not connected, retrying %d/%d for topic %s", i+1, MaxRetries, topic.TopicName)
				time.Sleep(RetryDelay)
				continue
			}
			return err
		}
		break
	}
	return nil
}

func (sse *SSEService) subscribeToTopic(queueName string, topic internal.SubscribedTopicsMap) error {
	handler, ok := reflect.TypeOf(sse).MethodByName(topic.TopicHandler)
	if !ok {
		return fmt.Errorf("Handler method %s not found for topic %s", topic.TopicHandler, topic.TopicName)
	}

	_, err := sse.brokerObj.Subscribe(topic.TopicName, func(p microBroker.Event) error {
		returnValues := handler.Func.Call([]reflect.Value{reflect.ValueOf(sse), reflect.ValueOf(p)})
		if err, ok := returnValues[0].Interface().(error); ok && err != nil {
			return err
		}
		return nil
	}, microBroker.Queue(queueName), rabbitmq.DurableQueue())

	if err != nil {
		log.Printf("Failed to subscribe to topic %s due to error: %v", topic.TopicName, err)
		return err
	}

	log.Printf("Successfully subscribed to topic %s | Handler: %s", topic.TopicName, topic.TopicHandler)
	return nil
}

func (sse *SSEService) HandleCaptureDeviceData(p microBroker.Event) error {

	log.Println("Process Capture Device Data Func Called")

	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		return fmt.Errorf("error unmarshalling data: %v", err)
	}

	deviceData := model.Device{}
	messageType := int32(data["messageType"].(float64))
	log.Println("Message Type is", messageType)
	if err := helper.InterfaceToStruct(data["data"], &deviceData); err != nil {
		return fmt.Errorf("error decoding data map: %v", err)
	}

	deviceDataMap, err := helper.StructToMap(deviceData)
	if err != nil {
		return fmt.Errorf("error converting device data to map: %v", err)
	}

	controller := controllers.GetControllerInstance(enum.CaptureScreenController, enum.MONGODB)
	captureScreenController := controller.(*controllers.CaptureScreenController)

	query := map[string]interface{}{
		"deviceName": deviceData.DeviceName,
	}

	deviceDataMap["isOnline"] = true
	deviceDataMap["lastOnline"] = time.Now().UTC()

	updateData := map[string]interface{}{}

	switch messageType {

	case int32(enum.PING_DEVICE):

		log.Println("Attempting to broadcast PING_DEVICE", messageType)
		log.Println("Message Data:", deviceDataMap)
		sse.sseHandler.Broadcast(map[string]interface{}{
			"message": deviceDataMap,
			"type":    "ping_device",
		})

		updateData = map[string]interface{}{
			"isOnline":    true,
			"memoryUsage": deviceData.MemoryUsage,
			"osName":      deviceData.OSName,
			"lastOnline":  time.Now().UTC(),
			"diskUsage":   deviceData.DiskUsage,
		}

		break

	case int32(enum.CAPTURE_SCREEN):

		// Use the SSE handler to broadcast
		log.Println("Attempting to broadcast CAPTURE_SCREEN", messageType)
		log.Println("Message Data:", deviceDataMap)
		sse.sseHandler.Broadcast(map[string]interface{}{
			"message": deviceDataMap,
			"type":    "capture_screen",
		})

		updateData = map[string]interface{}{
			"lastImage":   deviceData.LastImage,
			"isOnline":    true,
			"memoryUsage": deviceData.MemoryUsage,
			"osName":      deviceData.OSName,
			"lastOnline":  time.Now().UTC(),
			"diskUsage":   deviceData.DiskUsage,
		}

		break
	default:
		log.Println("No case found for", messageType)
	}

	e := captureScreenController.Update(query, updateData)
	if e != nil {
		log.Println("Error updating device")
	}
	return nil

}

// HandleVideoDownload processes video download requests from the queue
func (sse *SSEService) HandleVideoDownload(p microBroker.Event) error {
	log.Println("HandleVideoDownload called")

	data := make(map[string]interface{})
	if err := json.Unmarshal(p.Message().Body, &data); err != nil {
		return fmt.Errorf("error unmarshalling video download data: %v", err)
	}

	downloadId := data["downloadId"].(string)
	videoId := data["videoId"].(string)
	videoTitle := data["videoTitle"].(string)
	format := data["format"].(string)
	var quality string
	qualityStr, ok := data["quality"].(string)
	if !ok {
		quality = ""
	} else {
		quality = qualityStr
	}
	bitRate := data["bitRate"].(string)
	var youtubeURL string
	if urlStr, ok := data["youtubeURL"].(string); ok {
		youtubeURL = urlStr
	}

	log.Printf("🟡 Processing: %s (format: %s, quality: %s)", downloadId, format, quality)

	// Send initial status to SSE clients
	sse.sseHandler.BroadcastToRoute(fmt.Sprintf("download-%s", downloadId), map[string]interface{}{
		"downloadId": downloadId,
		"status":     "processing",
		"progress":   0,
		"message":    "Starting download...",
		"type":       "download_progress",
	})

	// Process the actual video download
	sse.downloadQueue.AddJob(downloadId, videoId, youtubeURL, format, quality, bitRate, videoTitle)

	return nil
}

// processVideoDownload handles the actual video download and S3 upload
func (sse *SSEService) processVideoDownload(downloadId, videoId, youtubeURL, format, quality, bitRate, videoTitle string) {
	routeKey := fmt.Sprintf("download-%s", downloadId)

	job := sse.downloadQueue.GetJob(downloadId)
	if job == nil {
		return
	}

	job.mu.Lock()
	job.Status = enum.DOWNLOADING
	job.mu.Unlock()

	// Sanitize title to ensure safe filesystem pathing and consistent output name
	sanitizedTitle := sanitizeFilename(videoTitle)

	onProgress := func(progress float64) {
		job.mu.Lock()
		job.Progress = int(progress)
		job.mu.Unlock()
	}

	log.Printf("[SSE-SERVICE:] Starting download for video %s with format %s, quality %s, bitrate %s", videoId, format, quality, bitRate)
	session, err := google.DownloadYoutubeVideoToBuffer(
		youtubeURL,
		videoId,
		format,
		quality,
		bitRate,
		sanitizedTitle,
		onProgress,
	)

	if err != nil {
		job.mu.Lock()
		job.Status = enum.SSEStreamEnum("error")
		job.mu.Unlock()
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"message":    fmt.Sprintf("Failed to start download: %v", err),
			"type":       "download_error",
		})
		return
	}

	if err := session.Wait(); err != nil {
		job.mu.Lock()
		job.Status = enum.SSEStreamEnum("error")
		job.mu.Unlock()
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"message":    fmt.Sprintf("Download failed: %v", err),
			"type":       "download_error",
		})
		return
	}

	filePath := session.GetFilePath()
	fileSize := session.GetFileSize()
	// Derive final filename using sanitized title and actual file extension
	var filename string
	{
		ext := ""
		if i := strings.LastIndex(filePath, "."); i != -1 && i+1 < len(filePath) {
			ext = filePath[i+1:]
		} else {
			// Fallback to requested format
			ext = format
		}
		filename = fmt.Sprintf("%s_%s.%s", sanitizedTitle, videoId, ext)
	}
	if sse.s3Service == nil {
		job.mu.Lock()
		job.Status = enum.SSEStreamEnum("error")
		job.mu.Unlock()
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"message":    "S3 service not initialized",
			"type":       "download_error",
		})
		return
	}

	// Open file as stream
	f, err := os.Open(filePath)
	if err != nil {
		job.mu.Lock()
		job.Status = enum.SSEStreamEnum("error")
		job.mu.Unlock()
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"message":    fmt.Sprintf("Failed to open file: %v", err),
			"type":       "download_error",
		})
		os.Remove(filePath)
		return
	}
	defer f.Close()

	// Generate S3 key
	s3Key := fmt.Sprintf("downloads/%s/%s", downloadId, filename)

	// Determine MIME type based on format
	mimeType := "application/octet-stream"
	if format == "mp3" {
		mimeType = "audio/mpeg"
	} else if format == "mp4" {
		mimeType = "video/mp4"
	}

	// Upload to S3 with 24-hour TTL using streaming
	presignedUrl, err := sse.s3Service.UploadFileStream(
		context.Background(),
		s3Key,
		f,
		mimeType,
		1440, // 24 hours
	)

	if err != nil {
		job.mu.Lock()
		job.Status = enum.SSEStreamEnum("error")
		job.mu.Unlock()
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"message":    fmt.Sprintf("Failed to upload to S3: %v", err),
			"type":       "download_error",
		})
		os.Remove(filePath)
		return
	}

	// Mark job as completed
	job.mu.Lock()
	job.FilePath = filePath
	job.FileSize = fileSize
	job.Status = enum.COMPLETED
	job.Progress = 100
	job.mu.Unlock()

	// Send completion message with download URL
	sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
		"downloadId":  downloadId,
		"type":        "download_complete",
		"status":      "completed",
		"progress":    100,
		"message":     "Download completed!",
		"filename":    filename,
		"fileSize":    fileSize,
		"downloadUrl": presignedUrl,
	})

	log.Printf("File uploaded to S3: %s", s3Key)

	// Clean up local file and S3 file after 1 hour
	time.AfterFunc(1*time.Hour, func() {
		// Delete local file
		if err := os.Remove(filePath); err != nil {
			log.Printf("Failed to delete local file %s: %v", filePath, err)
		} else {
			log.Printf("Cleaned up local file: %s", filePath)
		}

		// Delete S3 file
		if sse.s3Service != nil {
			if err := sse.s3Service.DeleteFile(context.Background(), s3Key); err != nil {
				log.Printf("Failed to delete S3 file %s: %v", s3Key, err)
			} else {
				log.Printf("Cleaned up S3 file: %s", s3Key)
			}
		}
	})
}

func (ls *SSEService) InitServiceConfig() {
	serviceConfig, er := internal.ReturnServiceConfig("sse-service")
	if er != nil {
		log.Println("Unable to read service config", er)
		return
	}
	ls.serviceConfig = serviceConfig.(internal.ServiceConfig)
}

func NewSSEService(serviceObj micro.Service, serviceName string) service.ServiceInterface {
	// Initialize base service via existing initializer
	base := (&SSEService{}).InitializeService(serviceObj, serviceName)
	sse := base.(*SSEService)

	sse.downloadQueue = NewDownloadQueue(3, sse)
	return sse
}

func (s *SSEService) registerRoutes() {
	// Direct streaming endpoint for large file downloads (with full format/quality control)
	s.router.HandleFunc("/stream/{downloadId}", s.handleDirectStream).Methods("GET")
}

func (sse *SSEService) Start(port string) error {

	godotenv.Load()
	log.Println("Starting SSE Service on Port:", port)

	sse.SubscribeTopics()

	// Create a new router and register the SSE handler
	sse.router = mux.NewRouter()

	// Generic events endpoint (existing functionality)
	sse.router.HandleFunc("/events", sse.sseHandler.ServeHTTP)

	// Route-specific endpoints for different projects
	sse.router.HandleFunc("/events/{route}", sse.handleRouteSpecificSSE)

	// Register additional routes (including streaming endpoint)
	sse.registerRoutes()

	sse.server = &http.Server{
		Addr:    ":" + port,
		Handler: sse.router,
	}

	if err := sse.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %v", err)
	}

	return nil

}

// Handle route-specific SSE connections
func (sse *SSEService) handleRouteSpecificSSE(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	route := vars["route"]

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	log.Printf("Route-specific SSE connection for route: %s", route)

	// Add client with route subscription using public method
	clientChan := sse.sseHandler.AddClientWithRoute(route)

	// Remove client when connection is closed
	defer func() {
		sse.sseHandler.RemoveClient(clientChan)
	}()

	for {
		select {
		case msg := <-clientChan:
			// Serialize the message to JSON
			jsonData, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Error marshalling message to JSON: %v", err)
				return
			}
			// Send the JSON data as a string
			_, err = fmt.Fprintf(w, "data: %s\n\n", jsonData)
			if err != nil {
				log.Printf("Error sending message to client: %v", err)
				return
			}
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// handleDirectStream streams video directly from yt-dlp to HTTP response
func (sse *SSEService) handleDirectStream(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	downloadId := vars["downloadId"]

	// Get download params from query
	videoId := r.URL.Query().Get("videoId")
	format := r.URL.Query().Get("format")
	quality := r.URL.Query().Get("quality")
	videoTitle := r.URL.Query().Get("videoTitle")
	youtubeURL := r.URL.Query().Get("youtubeURL")

	log.Printf("Direct stream request: downloadId=%s, videoId=%s, format=%s, quality=%s",
		downloadId, videoId, format, quality)

	// Validate required parameters
	if videoId == "" {
		http.Error(w, "videoId is required", http.StatusBadRequest)
		return
	}
	if format == "" {
		format = "mp4" // Default to mp4
	}

	// Sanitize title for logging
	sanitizedTitle := sanitizeFilename(videoTitle)
	if sanitizedTitle == "" {
		sanitizedTitle = videoId
	}

	routeKey := fmt.Sprintf("download-%s", downloadId)

	// Send initial progress via SSE
	sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
		"downloadId": downloadId,
		"progress":   5,
		"status":     "starting",
		"message":    "Initializing stream...",
		"type":       "download_progress",
	})

	// Create progress callback that sends updates via SSE
	progressCallback := func(progress float64) {
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"progress":   int(progress),
			"status":     "downloading",
			"type":       "download_progress",
		})
	}

	// Start yt-dlp streaming to stdout
	cmd, err := google.StreamYoutubeVideoToStdout(
		youtubeURL,
		videoId,
		format,
		quality,
		progressCallback,
	)

	if err != nil {
		log.Printf("Failed to start stream for %s: %v", downloadId, err)
		http.Error(w, fmt.Sprintf("Failed to start stream: %v", err), http.StatusInternalServerError)

		// Send error via SSE
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"message":    fmt.Sprintf("Failed to start stream: %v", err),
			"type":       "download_error",
		})
		return
	}

	// Set response headers for streaming
	mimeType := google.GetStreamMimeType(format)
	filename := fmt.Sprintf("%s.%s", sanitizedTitle, format)

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache")

	log.Printf("Starting stream for %s (%s)", filename, mimeType)

	// Pipe yt-dlp stdout directly to HTTP response
	cmd.Stdout = w

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start yt-dlp for %s: %v", downloadId, err)
		http.Error(w, fmt.Sprintf("Failed to start download: %v", err), http.StatusInternalServerError)

		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"message":    fmt.Sprintf("Failed to start download: %v", err),
			"type":       "download_error",
		})
		return
	}

	// Wait for yt-dlp to complete
	if err := cmd.Wait(); err != nil {
		log.Printf("Stream error for %s: %v", downloadId, err)

		// Send error via SSE (client may have already disconnected)
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"message":    fmt.Sprintf("Stream interrupted: %v", err),
			"type":       "download_error",
		})
		return
	}

	log.Printf("Stream completed successfully for %s", downloadId)

	// Send completion notification via SSE
	sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
		"downloadId": downloadId,
		"progress":   100,
		"status":     "completed",
		"message":    "Download completed successfully",
		"type":       "download_complete",
		"filename":   filename,
	})
}

func (sse *SSEService) Stop() error {
	// Stop the sse Gateway service
	log.Println("Stopping sse Gateway")
	return nil
}
