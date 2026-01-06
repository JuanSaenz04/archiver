package api

import (
	"net/http"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/JuanSaenz04/archiver/internal/queue"
	"github.com/labstack/echo/v4"
)

func (handler *Handler) HandleNewJob(c echo.Context) error {
	job := &models.CrawlRequest{}

	if err := c.Bind(job); err != nil {
		return respondWithError(http.StatusBadRequest, "Bad request", c)
	}

	jobId, err := queue.EnqueueCrawl(c.Request().Context(), handler.rdb, job.URL, job.Options)
	if err != nil {
		return respondWithError(http.StatusInternalServerError, "Failed to queue job", c)
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"job_id": jobId,
		"status": "pending",
	})
}

func (handler *Handler) HandleGetJobs(c echo.Context) error {
	jobs, err := handler.jobRepo.GetAllJobs(c.Request().Context())

	if err != nil {
		return respondWithError(http.StatusInternalServerError, "Internal server error", c)
	}

	return c.JSON(http.StatusOK, jobs)
}
