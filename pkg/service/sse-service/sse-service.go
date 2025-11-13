package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

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
	sse.downloadQueue.AddJob(downloadId, videoId, format, quality, bitRate, videoTitle)

	return nil
}

// processVideoDownload handles the actual video download with progress updates
func (sse *SSEService) processVideoDownload(downloadId, videoId, format, quality, bitRate, videoTitle string) {
	routeKey := fmt.Sprintf("download-%s", downloadId)

	job := sse.downloadQueue.GetJob(downloadId)
	if job == nil {
		return
	}

	job.mu.Lock()
	job.Status = enum.DOWNLOADING
	job.mu.Unlock()

	// Send immediate progress update to keep client connected
	sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
		"downloadId": downloadId,
		"status":     "downloading",
		"progress":   1,
		"message":    "Connecting to YouTube...",
		"type":       "download_progress",
	})

	streamStarted := false
	lastProgressUpdate := time.Now()

	// Send periodic heartbeats during initialization to keep client engaged
	heartbeatTicker := time.NewTicker(10 * time.Second)
	initializationDone := make(chan bool, 1)
	
	go func() {
		for {
			select {
			case <-heartbeatTicker.C:
				// Only send heartbeat if we're still in initialization phase
				if time.Since(lastProgressUpdate) > 5*time.Second {
					job.mu.Lock()
					currentProgress := job.Progress
					job.mu.Unlock()
					
					// Ensure progress is at least 3% to show activity
					if currentProgress < 3 {
						currentProgress = 3
					}
					
					sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
						"downloadId": downloadId,
						"status":     "downloading",
						"progress":   currentProgress,
						"message":    "Analyzing video formats...",
						"type":       "download_progress",
					})
				}
			case <-initializationDone:
				heartbeatTicker.Stop()
				return
			}
		}
	}()

	onProgress := func(progress float64) {
		job.mu.Lock()
		job.Progress = int(progress)
		job.mu.Unlock()
		lastProgressUpdate = time.Now()

		event := map[string]interface{}{
			"downloadId": downloadId,
			"status":     "downloading",
			"progress":   int(progress),
			"message":    fmt.Sprintf("Downloading... %.1f%%", progress),
			"type":       "download_progress",
		}
		sse.sseHandler.BroadcastToRoute(routeKey, event)

		if !streamStarted && progress >= 10 {
			streamStarted = true
			select {
			case initializationDone <- true:
			default:
			}
			log.Printf("🟢 Stream ready at %.1f%%", progress)
		}
	}

	// Sanitize title to ensure safe filesystem pathing and consistent output name
	sanitizedTitle := sanitizeFilename(videoTitle)

	// Send status update before calling yt-dlp
	sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
		"downloadId": downloadId,
		"status":     "downloading",
		"progress":   2,
		"message":    "Starting download...",
		"type":       "download_progress",
	})

	session, err := google.DownloadYoutubeVideoToBuffer(
		videoId,
		format,
		quality,
		bitRate,
		sanitizedTitle,
		onProgress,
	)
	
	// Stop heartbeat ticker when yt-dlp starts (or errors) - use select to avoid blocking
	select {
	case initializationDone <- true:
	default:
	}
	
	if err != nil {
		job.mu.Lock()
		job.Status = enum.SSEStreamEnum("error")
		job.mu.Unlock()
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"message":    fmt.Sprintf("Failed to start download: %v", err),
			"type":       "download_progress",
		})
		return
	}
	
	// Send status update when yt-dlp is running
	sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
		"downloadId": downloadId,
		"status":     "downloading",
		"progress":   5,
		"message":    "Video download in progress...",
		"type":       "download_progress",
	})

	job.mu.Lock()
	job.session = session
	job.mu.Unlock()

	if err := session.Wait(); err != nil {
		job.mu.Lock()
		job.Status = enum.SSEStreamEnum("error")
		job.mu.Unlock()
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"message":    fmt.Sprintf("Download failed: %v", err),
			"type":       "download_progress",
		})
		return
	}

	filePath := session.GetFilePath()
	fileSize := session.GetFileSize()
	filename := fmt.Sprintf("%s_%s.%s", sanitizedTitle, videoId, format)

	job.mu.Lock()
	job.FilePath = filePath
	job.FileSize = fileSize
	job.Status = enum.COMPLETED
	job.Progress = 100
	job.mu.Unlock()

	log.Printf("✅ Download complete: %s (%d bytes)", filePath, fileSize)

	sse.streamFileChunks(routeKey, downloadId, filePath, filename, fileSize)

	time.AfterFunc(1*time.Hour, func() {
		os.Remove(filePath)
		log.Printf("🗑️ Cleaned up: %s", filePath)
	})
}

func (sse *SSEService) streamFileChunks(routeKey, downloadId, filePath, filename string, totalSize int64) {
	const chunkSize = 65535

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("❌ Failed to open file: %v", err)
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"type":       "download_error",
			"message":    "Failed to read file",
		})
		return
	}
	defer file.Close()

	totalChunks := (totalSize + int64(chunkSize) - 1) / int64(chunkSize)
	log.Printf("📦 Streaming %d bytes in %d chunks", totalSize, totalChunks)

	buffer := make([]byte, chunkSize)
	chunkIndex := 0

	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	// Channel to signal when file reading is done
	fileDone := make(chan error, 1)
	chunkChan := make(chan struct {
		index int
		data  string
		size  int
	}, 10) // buffer chunks for concurrent processing

	// Goroutine to read file chunks
	go func() {
		defer close(chunkChan)
		for {
			n, err := file.Read(buffer)
			if n == 0 || err == io.EOF {
				fileDone <- nil
				return
			}
			if err != nil {
				fileDone <- err
				return
			}

			chunkData := base64.StdEncoding.EncodeToString(buffer[:n])
			chunkChan <- struct {
				index int
				data  string
				size  int
			}{
				index: chunkIndex,
				data:  chunkData,
				size:  n,
			}
			chunkIndex++
		}
	}()

	chunksSent := 0
	for {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				err := <-fileDone
				if err != nil {
					log.Printf("❌ Read error: %v", err)
					return
				}

				// File read complete, send final message
				sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
					"downloadId": downloadId,
					"type":       "download_complete",
					"status":     "completed",
					"progress":   100,
					"message":    "Download completed!",
					"filename":   filename,
					"fileSize":   totalSize,
				})
				log.Printf("✅ File streaming complete (%d chunks sent)", chunksSent)
				return
			}

			sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
				"downloadId":  downloadId,
				"type":        "file_chunk",
				"chunkIndex":  chunk.index,
				"totalChunks": totalChunks,
				"chunkData":   chunk.data,
				"chunkSize":   chunk.size,
			})

			chunksSent++
			if chunksSent%100 == 0 {
				log.Printf("📤 Chunk %d/%d sent", chunksSent, totalChunks)
			}

		// Send heartbeat every 15 seconds to keep connection alive
		case <-heartbeatTicker.C:
			sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
				"type": "heartbeat",
			})
			log.Printf("Heartbeat sent (chunks: %d/%d)", chunksSent, totalChunks)
		}
	}
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

func (sse *SSEService) Stop() error {
	// Stop the sse Gateway service
	log.Println("Stopping sse Gateway")
	return nil
}
