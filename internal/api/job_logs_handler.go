package api

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
)

var logCursorPattern = regexp.MustCompile(`^[0-9]{1,20}-[0-9]{1,20}$`)

func (h *Handler) HandleJobLogs(c *echo.Context) error {
	id := c.Param("jobId")
	if _, err := uuid.Parse(id); err != nil {
		return echo.ErrBadRequest
	}
	cursor := c.Request().Header.Get("Last-Event-ID")
	if cursor != "" && !logCursorPattern.MatchString(cursor) {
		return echo.ErrBadRequest
	}
	if cursor != "" {
		parts := strings.Split(cursor, "-")
		for i, part := range parts {
			n, err := strconv.ParseUint(part, 10, 64)
			if err != nil {
				return echo.ErrBadRequest
			}
			parts[i] = strconv.FormatUint(n, 10)
		}
		cursor = strings.Join(parts, "-")
	}
	select {
	case h.logSlots <- struct{}{}:
		defer func() { <-h.logSlots }()
	default:
		return echo.NewHTTPError(http.StatusTooManyRequests, "Too many log viewers")
	}
	ctx := c.Request().Context()
	logs := h.jobs.Logs()
	status, err := logs.Status(ctx, id)
	if errors.Is(err, redis.Nil) {
		return echo.ErrNotFound
	}
	if err != nil {
		return echo.ErrServiceUnavailable
	}
	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	defer controller.SetWriteDeadline(time.Time{})
	send := func(event, id string, data any) error {
		payload, err := json.Marshal(data)
		if err != nil {
			return err
		}
		_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if id != "" {
			if _, err = fmt.Fprintf(w, "id: %s\n", id); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
			return err
		}
		return controller.Flush()
	}
	if err := send("status", "", status); err != nil {
		return nil
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	lastHeartbeat := time.Now()
	for {
		// Read status first: terminal status is written only after the log session drains.
		status, err = logs.Status(ctx, id)
		if err != nil {
			return nil
		}
		entries, err := logs.Read(ctx, id, cursor)
		if err != nil {
			return nil
		}
		if len(entries) > 0 {
			missing, err := logs.HistoryMissing(ctx, id, cursor)
			if err != nil {
				return nil
			}
			if missing {
				if err := send("gap", "", "Older log entries are no longer available"); err != nil {
					return nil
				}
			}
			cursor = entries[len(entries)-1].ID
			if err := send("logs", cursor, entries); err != nil {
				return nil
			}
			continue
		}
		if status == "completed" || status == "failed" {
			available, err := logs.Available(ctx, id)
			if err != nil {
				return nil
			}
			_ = send("done", "", map[string]any{"status": status, "unavailable": !available})
			return nil
		}
		if cursor == "" {
			cursor = "0-0"
		}
		if time.Since(lastHeartbeat) >= 15*time.Second {
			if err := send("status", "", status); err != nil {
				return nil
			}
			lastHeartbeat = time.Now()
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
