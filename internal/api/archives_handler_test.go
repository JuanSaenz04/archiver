package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/JuanSaenz04/archiver/internal/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHandleGetArchives(t *testing.T) {
	// 1. Setup temporary directory for archives
	tempDir := t.TempDir()

	// 2. Create dummy archive files
	expectedArchives := []string{"archive1.wacz", "archive2.wacz"}
	for _, name := range expectedArchives {
		file, err := os.Create(filepath.Join(tempDir, name))
		if err != nil {
			t.Fatalf("Failed to create dummy archive file: %v", err)
		}
		file.Close()
	}
	// Create a non-wacz file to ensure it's ignored
	ignoredFile, err := os.Create(filepath.Join(tempDir, "ignored.txt"))
	if err != nil {
		t.Fatalf("Failed to create ignored file: %v", err)
	}
	ignoredFile.Close()

	// 3. Initialize Handler
	// We pass nil for redis client since GetArchives doesn't use it
	handler := &Handler{
		archivesDir: tempDir,
	}

	// 4. Setup Echo context
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/archives", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// 5. Call the handler
	if assert.NoError(t, handler.HandleGetArchives(c)) {
		// 6. Assertions
		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string][]models.Archive
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)

		archives := response["archives"]
		assert.Len(t, archives, 2)

		// Verify the names are correct (order might vary depending on OS, so we check existence)
		archiveNames := make([]string, len(archives))
		for i, a := range archives {
			archiveNames[i] = a.Name
		}
		assert.ElementsMatch(t, expectedArchives, archiveNames)
	}
}

func TestHandleGetArchive(t *testing.T) {
	tempDir := t.TempDir()
	archiveName := "test.wacz"
	content := []byte("dummy wacz content")
	
	err := os.WriteFile(filepath.Join(tempDir, archiveName), content, 0644)
	if err != nil {
		t.Fatalf("Failed to create dummy archive: %v", err)
	}

	handler := &Handler{archivesDir: tempDir}
	e := echo.New()

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/archives/"+archiveName, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues(archiveName)

		if assert.NoError(t, handler.HandleGetArchive(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, content, rec.Body.Bytes())
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/archives/nonexistent.wacz", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("archiveName")
		c.SetParamValues("nonexistent.wacz")

		// HandleGetArchive calls c.File which might return error or handle it.
		// In Echo, if c.File fails to find the file, it returns an error.
		err := handler.HandleGetArchive(c)
		assert.NoError(t, err) // Our handler catches the error and responds
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
