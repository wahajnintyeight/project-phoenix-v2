package google

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type StreamSession struct {
	filePath string
	fileSize int64
	ctx      context.Context
	cancel   context.CancelFunc
	cmd      *exec.Cmd
	done     chan error
	mu       sync.RWMutex
}

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
			if cmd, err := validateAndBuild(parts[0], all); err == nil {
				return cmd, nil
			} else {
				logger.Printf("YT_DLP_BIN invalid (%s), falling back: %v", parts[0], err)
			}
		}
	}

	// Try common install location
	if cmd, err := validateAndBuild("/usr/local/bin/yt-dlp", args); err == nil {
		return cmd, nil
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
func (yt *StreamSession) runYtDlp(cmd *exec.Cmd, progressCallback ProgressCallback, videoId string, videoTitle string, videoFormat string) {

	stderror, e := cmd.StderrPipe()
	if e != nil {
		yt.done <- fmt.Errorf("stderr pipe error: %w", e)
		close(yt.done)
		return
	}

	logger.Printf("Running: %s %s", cmd.Path, strings.Join(cmd.Args[1:], " "))

	// Send initial callback to indicate yt-dlp started
	if progressCallback != nil {
		progressCallback(10) // 10% indicates yt-dlp has started
	}

	if err := cmd.Start(); err != nil {
		logger.Printf("start yt-dlp: %v", err)
		yt.done <- err
		close(yt.done)
		return
	}
	var stderrBuf bytes.Buffer
	go func() {
		scanner := bufio.NewScanner(stderror)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")
			logger.Printf("YT-DLP: %s", line)

			// Send progress callback on retry/format selection messages to keep client engaged
			if strings.Contains(line, "Retrying") || strings.Contains(line, "format") || strings.Contains(line, "extracting") {
				if progressCallback != nil {
					// Small progress update to show activity
					progressCallback(15) // 15% during format selection/retry
				}
			}

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

	if err := yt.cmd.Wait(); err != nil {
		logger.Printf(" yt-dlp exited with error: %v", err)
		logger.Printf("STDERR: %s", stderrBuf.String())
		yt.done <- fmt.Errorf("yt-dlp failed: %w", err)
		close(yt.done)
		return
	}

	// Locate actual output file (handles cases like mp4.mp3 produced by post-processing)
	pattern := fmt.Sprintf("/tmp/yt-downloads/%s_%s*", videoTitle, videoId)
	logger.Printf(" Searching for files matching: %s", pattern)

	matches, gerr := filepath.Glob(pattern)
	if gerr != nil || len(matches) == 0 {
		logger.Printf(" No files found matching pattern")
		// List directory contents for debugging
		files, _ := os.ReadDir("/tmp/yt-downloads")
		logger.Printf(" Files in /tmp/yt-downloads:")
		for _, f := range files {
			if info, err := f.Info(); err == nil {
				logger.Printf("   - %s (size: %d)", f.Name(), info.Size())
			}
		}
		yt.done <- fmt.Errorf("file not found")
		close(yt.done)
		return
	}

	filePath := matches[0]
	logger.Printf(" Found file: %s", filePath)

	stat, err := os.Stat(filePath)
	if err != nil {
		yt.done <- fmt.Errorf("stat failed: %w", err)
		close(yt.done)
		return
	}

	yt.mu.Lock()
	yt.filePath = filePath
	yt.fileSize = stat.Size()
	yt.mu.Unlock()

	logger.Printf(" Download complete: %s (%d bytes)", filePath, stat.Size())
	yt.done <- nil
	close(yt.done)
}

// Wait blocks until the download finishes and returns error if any
func (yt *StreamSession) Wait() error {
	if yt == nil || yt.done == nil {
		return fmt.Errorf("invalid session")
	}
	err, ok := <-yt.done
	if !ok {
		return nil
	}
	return err
}

// GetFilePath returns the downloaded file path
func (yt *StreamSession) GetFilePath() string {
	yt.mu.RLock()
	defer yt.mu.RUnlock()
	return yt.filePath
}

// GetFileSize returns the downloaded file size
func (yt *StreamSession) GetFileSize() int64 {
	yt.mu.RLock()
	defer yt.mu.RUnlock()
	return yt.fileSize
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

func DownloadYoutubeVideoToBuffer(videoId string, format string, quality string, bitRate string, videoTitle string, progressCallback ProgressCallback) (*StreamSession, error) {

	downloadDir := "/tmp/yt-downloads"
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download dir: %w", err)
	}
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoId)
	
	logger.Printf("Downloading video %s in format %s, quality %s, bitrate %s", videoId, format, quality, bitRate)

	var args []string
    // Common args to reduce throttling and improve reliability
    commonArgs := []string{
        "--no-playlist",
        "--newline",
        "--no-mtime",
        "--downloader", "aria2c",
        "--downloader-args", "aria2c:-x 16 -k 1M",
        "--force-ipv4",
        "--concurrent-fragments", "5",
        "--remote-components", "ejs:github",
        "--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
    }

	// If a local deno binary exists in the service-configs utils directory, tell yt-dlp to use it
	denoPath := "/app/internal/service-configs/sse-service/utils/deno"
	if info, err := os.Stat(denoPath); err == nil && info.Mode()&0111 != 0 {
		commonArgs = append(commonArgs, "--js-runtimes", fmt.Sprintf("deno:%s", denoPath))
	}

    if format == "mp3" {
        args = append([]string{
            "--extract-audio",
            "--audio-format", "mp3",
            "--audio-quality", getBitrate(bitRate),
            "--output", fmt.Sprintf("/tmp/yt-downloads/%s_%s.%%(ext)s", videoTitle, videoId),
            videoURL,
        }, commonArgs...)
    } else {
        formatStr := buildVideoFormatString(format, quality)
        // Use progressive format for better stdout streaming
        args = append([]string{
            "--format", formatStr,
            "--output", fmt.Sprintf("/tmp/yt-downloads/%s_%s.%%(ext)s", videoTitle, videoId),
            videoURL,
        }, commonArgs...)
    }

	cmd, err := buildYtDlpCmd(args...)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	session := &StreamSession{
		ctx:    ctx,
		cancel: cancel,
		cmd:    cmd,
		done:   make(chan error, 1),
	}

	go session.runYtDlp(cmd, progressCallback, videoId, videoTitle, format)
	if err != nil {
		return nil, err // already includes stderr
	}

	return session, nil
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

func buildVideoFormatString(format, quality string) string {
	logger.Printf("🎬 buildVideoFormatString called: format=%s, quality=%s", format, quality)
	
	// Default to best if quality not specified
	if quality == "" || quality == "best" {
		quality = "best"
	}

	// Map quality strings to height constraints
	heightMap := map[string]string{
		"144p":  "144",
		"240p":  "240",
		"360p":  "360",
		"480p":  "480",
		"720p":  "720",
		"1080p": "1080",
		"1440p": "1440",
		"2160p": "2160",
	}

	height := heightMap[quality]
	if height == "" {
		// Try to extract height from quality string (e.g., "360" -> "360")
		if strings.Contains(quality, "p") {
			// Already in format like "360p", should have been in map
			logger.Printf("⚠️ Quality '%s' not recognized, falling back to best", quality)
			height = "best"
		} else if quality != "best" {
			// Might be just a number like "360"
			height = quality
		} else {
			height = "best"
		}
	}

	var formatStr string

	switch format {
	case "mp4":
		if height == "best" {
			formatStr = "best[ext=mp4]/best"
		} else {
			// Get best video+audio combo for specified height
			formatStr = fmt.Sprintf("best[height<=?%s][ext=mp4]+best[ext=m4a]/best[height<=?%s]/best[ext=mp4]+best", height, height)
		}

	case "webm":
		if height == "best" {
			formatStr = "best[ext=webm]/best"
		} else {
			formatStr = fmt.Sprintf("best[height<=?%s][ext=webm]/best[height<=?%s]/best[ext=webm]", height, height)
		}

	case "mkv":
		if height == "best" {
			formatStr = "best[ext=mkv]/best"
		} else {
			formatStr = fmt.Sprintf("best[height<=?%s][ext=mkv]/best[height<=?%s]/best[ext=mkv]", height, height)
		}

	default:
		// Default to mp4 best if format not recognized
		formatStr = "best[ext=mp4]/best"
	}

	logger.Printf("✅ Built format string: %s (height=%s)", formatStr, height)
	return formatStr
}

// buildStreamFormatString creates format string optimized for streaming
