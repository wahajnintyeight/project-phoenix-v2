package google

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"project-phoenix/v2/internal/model"
	"strings"

	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Google interface {
	SearchYoutube(searchQuery string)
}

var (
	logger = log.New(os.Stdout, "GOOGLE: ", log.LstdFlags)
)

func GetAPIKey() string {
	godotenv.Load()
	API_KEY := os.Getenv("GOOGLE_API_KEY")
	return API_KEY
}

func SearchYoutube(searchQuery string, maxResults int64, nextPage string, prevPage string) (interface{}, error) {

	ctx := context.Background()
	logger.Println("Creating YouTube service")
	service, err := youtube.NewService(ctx, option.WithAPIKey(GetAPIKey()))
	if err != nil {
		log.Fatalf("Error creating YouTube service: %v", err)
	}
	api := service.Search.List([]string{"snippet"}).Q(searchQuery).MaxResults(maxResults).Type("video").Order("relevance")

	if nextPage != "" {
		api.PageToken(nextPage)
	}
	res, e := api.Do()
	if e != nil {
		return nil, e
	} else {
		return map[string]interface{}{
			"items":     res.Items,
			"totalPage": res.PageInfo.TotalResults,
			"nextPage":  res.NextPageToken,
			"prevPage":  res.PrevPageToken,
		}, nil
	}

}
func DownloadYoutubeVideoToBuffer(videoId string, format string, bitRate string) ([]byte, string, error) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoId)
	formatString := buildFormatString(model.GoogleDownloadVideoRequestModel{
		Format:  format,
		BitRate: bitRate,
	})

	var cmd *exec.Cmd
	if format == "mp3" {
		cmd = exec.Command("python3", "-m", "yt_dlp",
			"--extract-audio",
			"--audio-format", "mp3",
			"--audio-quality", getBitrate(bitRate),
			"--output", "-",
			"--no-playlist",
			"--quiet",
			videoURL,
		)
	} else {
		cmd = exec.Command("python3", "-m", "yt_dlp",
			"--format", formatString,
			"--output", "-",
			"--no-playlist",
			"--quiet",
			videoURL,
		)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("failed to start yt-dlp: %v", err)
	}

	// Read all output into buffer
	fileContent, err := io.ReadAll(stdout)
	if err != nil {
		cmd.Process.Kill()
		return nil, "", fmt.Errorf("failed to read video: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, "", fmt.Errorf("yt-dlp process failed: %v", err)
	}

	filename := fmt.Sprintf("video_%s.%s", videoId, format)
	return fileContent, filename, nil
}
// DownloadYoutubeVideo downloads a YouTube video using yt-dlp and returns file as base64
func DownloadYoutubeVideo(videoId string, format string, bitRate string) (interface{}, error) {
	// Create temporary directory for downloads
	tempDir, err := os.MkdirTemp("", "youtube_download_")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Construct YouTube URL
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoId)

	// Output template for yt-dlp (will create filename with title and extension)
	outputTemplate := filepath.Join(tempDir, "%(title)s.%(ext)s")

	// yt-dlp command with options for best quality video+audio
	cmd := exec.Command("yt-dlp",
		"--format", "best[ext=mp4]/best", // Prefer mp4, fallback to best available
		"--output", outputTemplate,
		"--no-playlist", // Only download single video
		videoURL,
	)

	// Execute yt-dlp command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp failed: %v, output: %s", err, string(output))
	}

	// Find the downloaded file
	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp directory: %v", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files downloaded")
	}

	// Get the first (and should be only) downloaded file
	downloadedFile := files[0]
	filePath := filepath.Join(tempDir, downloadedFile.Name())

	// Read file content
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read downloaded file: %v", err)
	}

	// Convert to base64 for transmission
	base64Content := base64.StdEncoding.EncodeToString(fileContent)

	// Get file extension for MIME type
	ext := strings.ToLower(filepath.Ext(downloadedFile.Name()))
	mimeType := getMimeType(ext)

	return map[string]interface{}{
		"videoId":     videoId,
		"status":      "completed",
		"message":     "Video downloaded successfully",
		"filename":    downloadedFile.Name(),
		"fileSize":    len(fileContent),
		"mimeType":    mimeType,
		"fileContent": base64Content, // Base64 encoded file content
	}, nil
}

// getMimeType returns appropriate MIME type based on file extension
func getMimeType(ext string) string {
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".flv":
		return "video/x-flv"
	default:
		return "video/mp4" // Default fallback
	}
}

// buildFormatString creates yt-dlp format string based on request
func buildFormatString(req model.GoogleDownloadVideoRequestModel) string {
	format := req.Format
	quality := req.BitRate
	if quality == "320k" {
		quality = "best"
	}

	formatMap := map[string]map[string]string{
		"mp4": {
			"best":  "best[ext=mp4]/bestvideo[ext=mp4]+bestaudio[ext=m4a]/best",
			"worst": "worst[ext=mp4]",
			"720":   "best[height<=720][ext=mp4]",
			"1080":  "best[height<=1080][ext=mp4]",
			"4k":    "best[height<=2160][ext=mp4]",
		},
		"webm": {
			"best":  "best[ext=webm]",
			"worst": "worst[ext=webm]",
			"720":   "best[height<=720][ext=webm]",
			"1080":  "best[height<=1080][ext=webm]",
		},
	}

	if formats, exists := formatMap[format]; exists {
		if formatStr, exists := formats[quality]; exists {
			return formatStr
		}
	}
	return "best"
}

// getBitrate returns valid bitrate for audio downloads
func getBitrate(bitrate string) string {
	validBitrates := map[string]string{
		"128k": "128K",
		"192k": "192K",
		"256k": "256K",
		"320k": "320K",
	}
	if b, exists := validBitrates[strings.ToLower(bitrate)]; exists {
		return b
	}
	return "192K"
}

// getStreamMimeType returns MIME type for streaming based on format
func GetStreamMimeType(format string) string {
	mimeTypes := map[string]string{
		"mp4":  "video/mp4",
		"webm": "video/webm",
		"mp3":  "audio/mpeg",
		"m4a":  "audio/mp4",
		"ogg":  "audio/ogg",
	}
	if mime, exists := mimeTypes[format]; exists {
		return mime
	}
	return "application/octet-stream"
}
