package google

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Google interface {
	SearchYoutube(searchQuery string)
}

type ProgressCallback func(progress float64)

var (
	logger        = log.New(os.Stdout, "GOOGLE: ", log.LstdFlags)
	progressRegex = regexp.MustCompile(`\[download\]\s+(\d+\.\d+)%`)
)

// buildYtDlpCmd resolves yt-dlp binary and builds command
func buildYtDlpCmd(args ...string) (*exec.Cmd, error) {
	// Try environment variable first

	if cookieFile := strings.TrimSpace(os.Getenv("YT_DLP_COOKIES")); cookieFile != "" {
		if _, err := os.Stat(cookieFile); err == nil {
			args = append([]string{"--cookies", cookieFile}, args...)
		} else {
			logger.Printf("YT_DLP_COOKIES set but unreadable: %v", err)
		}
	}

	binstr := strings.TrimSpace(os.Getenv("YT_DLP_BIN"))
	if binstr != "" {
		parts := strings.Fields(binstr)
		if len(parts) > 0 {
			all := append(parts[1:], args...)
			return exec.Command(parts[0], all...), nil
		}
	}

	// Try yt-dlp in PATH
	if path, err := exec.LookPath("yt-dlp"); err == nil {
		return exec.Command(path, args...), nil
	}

	// Try python module
	for _, py := range []string{"python3", "python"} {
		if path, err := exec.LookPath(py); err == nil {
			all := append([]string{"-m", "yt_dlp"}, args...)
			return exec.Command(path, all...), nil
		}
	}

	return nil, fmt.Errorf("yt-dlp not found in PATH or as python module")
}

// runYtDlp executes yt-dlp command and captures output
func runYtDlp(cmd *exec.Cmd, progressCallback ProgressCallback) ([]byte, string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", fmt.Errorf("stdout pipe: %v", err)
	}

	stderror, e := cmd.StderrPipe()
	if e != nil {
		return nil, "", fmt.Errorf("stderr pipe: %v", e)
	}

	logger.Printf("Running: %s %s", cmd.Path, strings.Join(cmd.Args[1:], " "))

	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("start yt-dlp: %v", err)
	}
	var stderrBuf bytes.Buffer
	go func() {
		scanner := bufio.NewScanner(stderror)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")

			// Extract progress percentage
			if matches := progressRegex.FindStringSubmatch(line); len(matches) > 1 {
				if percent, err := strconv.ParseFloat(matches[1], 64); err == nil {
					// Scale to 25-95% range (reserve 0-25 for queuing, 95-100 for finalization)
					scaledProgress := 25 + (percent * 0.7)
					if progressCallback != nil {
						progressCallback(scaledProgress)
					}
				}
			}
		}
	}()

	data, readErr := io.ReadAll(stdout)
	waitErr := cmd.Wait()

	if readErr != nil {
		_ = cmd.Process.Kill()
		return nil, "", fmt.Errorf("read stdout: %v, stderr: %s", readErr, stderrBuf.String())
	}

	if waitErr != nil {
		return nil, "", fmt.Errorf("yt-dlp failed: %v, stderr: %s", waitErr, stderrBuf.String())
	}

	return data, "Success", nil
}

func validateAndBuild(binPath string, args []string) (*exec.Cmd, error) {
	info, err := os.Stat(binPath)
	if err != nil {
		return nil, err
	}

	// Check if executable
	if info.Mode()&0111 == 0 {
		return nil, fmt.Errorf("binary not executable: %s", binPath)
	}

	return exec.Command(binPath, args...), nil
}
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

func DownloadYoutubeVideoToBuffer(videoId string, format string, bitRate string, progressCallback ProgressCallback) ([]byte, string, error) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoId)

	var args []string
	if format == "mp3" {
		args = []string{
			"--extract-audio",
			"--audio-format", "mp3",
			"--audio-quality", getBitrate(bitRate),
			"--no-playlist",
			"--newline",
			"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			"--output", "-",
			videoURL,
		}
	} else {
		// Use progressive format for better stdout streaming
		args = []string{
			"--format", "best*[vcodec*=avc1][acodec*=mp4a][ext=mp4]/best[ext=mp4]/best",
			"--no-playlist",
			"--newline",
			"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			"--output", "-",
			videoURL,
		}
	}

	cmd, err := buildYtDlpCmd(args...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve yt-dlp: %v", err)
	}

	data, _, err := runYtDlp(cmd, progressCallback)
	if err != nil {
		return nil, "", err // already includes stderr
	}

	filename := fmt.Sprintf("video_%s.%s", videoId, format)
	return data, filename, nil
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

// buildStreamFormatString creates format string optimized for streaming
func buildStreamFormatString(format, quality string) string {
	if quality == "" {
		quality = "best"
	}

	formatMap := map[string]map[string]string{
		"mp4": {
			"best":  "best*[vcodec*=avc1][acodec*=mp4a][ext=mp4]/best[ext=mp4]/best",
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
