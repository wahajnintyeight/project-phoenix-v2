package helper

import (
	"fmt"
	"log"
	"time"
)

// LogContext contains contextual information for structured logging
type LogContext struct {
	ServiceName   string
	Operation     string
	CorrelationID string
}

// LogInfo logs an informational message with structured context
func LogInfo(ctx LogContext, message string, args ...interface{}) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	formattedMsg := fmt.Sprintf(message, args...)

	if ctx.CorrelationID != "" {
		log.Printf("[%s] [INFO] [%s] [%s] [correlation_id=%s] %s",
			timestamp, ctx.ServiceName, ctx.Operation, ctx.CorrelationID, formattedMsg)
	} else {
		log.Printf("[%s] [INFO] [%s] [%s] %s",
			timestamp, ctx.ServiceName, ctx.Operation, formattedMsg)
	}
}

// LogError logs an error message with structured context
func LogError(ctx LogContext, message string, err error, args ...interface{}) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	formattedMsg := fmt.Sprintf(message, args...)

	if ctx.CorrelationID != "" {
		log.Printf("[%s] [ERROR] [%s] [%s] [correlation_id=%s] %s: %v",
			timestamp, ctx.ServiceName, ctx.Operation, ctx.CorrelationID, formattedMsg, err)
	} else {
		log.Printf("[%s] [ERROR] [%s] [%s] %s: %v",
			timestamp, ctx.ServiceName, ctx.Operation, formattedMsg, err)
	}
}

// GenerateCorrelationID generates a simple correlation ID based on timestamp
func GenerateCorrelationID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
