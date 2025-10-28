package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
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

	// "sync"

	"github.com/go-micro/plugins/v4/broker/rabbitmq"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go-micro.dev/v4"
	microBroker "go-micro.dev/v4/broker"
	// "log"
)

type DownloadCleanupJob struct {
	Key       string
	ExpiresAt time.Time
}

type SSEService struct {
	service            micro.Service
	router             *mux.Router
	server             *http.Server
	serviceConfig      internal.ServiceConfig
	subscribedServices []internal.SubscribedServices
	brokerObj          microBroker.Broker
	sseHandler         *handler.SSERequestHandler
	s3Service          *aws.S3Service
	cleanupScheduler   chan DownloadCleanupJob
	cleanupJobs        map[string]chan bool // downloadId -> stop channel
}

// cleanupWorker processes file deletions on schedule
func (sse *SSEService) cleanupWorker() {
	for job := range sse.cleanupScheduler {
		go func(j DownloadCleanupJob) {
			timer := time.NewTimer(time.Until(j.ExpiresAt))
			defer timer.Stop()

			<-timer.C

			if sse.s3Service == nil {
				log.Printf("Cleanup skipped, s3Service is nil for key: %s", j.Key)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := sse.s3Service.DeleteFile(ctx, j.Key); err != nil {
				log.Printf(" Failed to delete S3 file %s: %v", j.Key, err)
			} else {
				log.Printf(" Auto-deleted S3 file: %s", j.Key)
			}
		}(job)
	}
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
	format := data["format"].(string)
	bitRate := data["bitRate"].(string)

	log.Printf("Processing video download - ID: %s, VideoID: %s", downloadId, videoId)

	// Send initial status to SSE clients
	sse.sseHandler.BroadcastToRoute(fmt.Sprintf("download-%s", downloadId), map[string]interface{}{
		"downloadId": downloadId,
		"status":     "processing",
		"progress":   0,
		"message":    "Starting video download...",
		"type":       "download_progress",
	})

	// Process the actual video download
	go sse.processVideoDownload(downloadId, videoId, format, bitRate)

	return nil
}

// processVideoDownload handles the actual video download with progress updates
func (sse *SSEService) processVideoDownload(downloadId, videoId string, format string, bitRate string) {
	routeKey := fmt.Sprintf("download-%s", downloadId)
	ctx := context.Background()

	// Send progress update
	progressCallback := func(progress float64) {
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "downloading",
			"progress":   int(progress),
			"message":    fmt.Sprintf("Downloading... %.1f%%", progress),
			"type":       "download_progress",
		})
	}

	fileContent, filename, err := google.DownloadYoutubeVideoToBuffer(videoId, format, bitRate, progressCallback)
	if err != nil {
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"progress":   0,
			"message":    fmt.Sprintf("Download failed: %v", err),
			"type":       "download_error",
		})
		log.Printf(" Download %s failed: %v", downloadId, err)
		return
	}


	// Ensure S3 service exists
	if sse.s3Service == nil {
		if s3svc, _, err := aws.NewS3ServiceFromEnv(); err != nil {
			sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
				"downloadId": downloadId,
				"status":     "error",
				"progress":   0,
				"message":    fmt.Sprintf("S3 init failed: %v", err),
				"type":       "download_error",
			})
			log.Printf(" S3 init failed for %s: %v", downloadId, err)
			return
		} else {
			sse.s3Service = s3svc
		}
	}

	// Build S3 key and upload
	key := fmt.Sprintf("downloads/%s/%s", downloadId, filename)
	mimeType := google.GetStreamMimeType(format)

	size := len(fileContent)
	url, upErr := sse.s3Service.UploadFile(ctx, key, fileContent, mimeType, 60)
	if upErr != nil {
		sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
			"downloadId": downloadId,
			"status":     "error",
			"progress":   0,
			"message":    fmt.Sprintf("Upload failed: %v", upErr),
			"type":       "download_error",
		})
		log.Printf(" S3 upload failed for %s: %v", downloadId, upErr)
		return
	}

	// Schedule auto-deletion after 60 minutes
	expiresAt := time.Now().Add(60 * time.Minute)
	if sse.cleanupScheduler != nil {
		sse.cleanupScheduler <- DownloadCleanupJob{Key: key, ExpiresAt: expiresAt}
	}

	// Clear the buffer
	fileContent = nil

	// Emit completion with URL and expiry info
	sse.sseHandler.BroadcastToRoute(routeKey, map[string]interface{}{
		"downloadId":  downloadId,
		"status":      "completed",
		"progress":    100,
		"message":     "Download completed successfully!",
		"type":        "download_complete",
		"filename":    filename,
		"fileSize":    size,
		"mimeType":    mimeType,
		"downloadUrl": url,
		"expiresIn":   3600, // seconds
		"expiresAt":   expiresAt.Unix(),
	})

	log.Printf(" Download %s completed and uploaded to S3: %s", downloadId, url)
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

    // Initialize S3 service from envs (once)
    if sse.s3Service == nil {
        if s3svc, _, err := aws.NewS3ServiceFromEnv(); err != nil {
            log.Printf("S3 init from env failed: %v", err)
        } else {
            sse.s3Service = s3svc
        }
    }

    // Start cleanup worker
    if sse.cleanupScheduler == nil {
        sse.cleanupScheduler = make(chan DownloadCleanupJob, 100)
        sse.cleanupJobs = make(map[string]chan bool)
        go sse.cleanupWorker()
    }

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
