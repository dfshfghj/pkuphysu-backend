package utils

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRespondErrorAllowsNilError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	RespondError(c, 404, "NotFound", nil)

	if recorder.Code != 404 {
		t.Fatalf("RespondError() status = %d, want 404", recorder.Code)
	}
}
