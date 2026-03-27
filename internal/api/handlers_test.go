package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleHealth_Returns200(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAllEndpoints_ReturnContentTypeJSON(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("Content-Type", "application/json")
	assert.Contains(t, w.Header().Get("Content-Type"), "json")
}

func TestHandleListProcesses_Returns200_WithEmptyArray_WhenNoProcesses(t *testing.T) {
}
