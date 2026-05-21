//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/search-service/mocks"
	"go.uber.org/mock/gomock"
)

func TestSearchHandler_SearchMerchants(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockSearchServiceInterface(ctrl)
	h := NewSearchHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/search/merchants", nil)
	w := httptest.NewRecorder()

	// Call the handler
	h.SearchMerchants(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", res.Status)
	}
}
