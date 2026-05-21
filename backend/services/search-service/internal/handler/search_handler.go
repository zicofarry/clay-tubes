package handler

import (
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/search-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

type SearchHandler struct {
	service service.SearchServiceInterface
}

func NewSearchHandler(svc service.SearchServiceInterface) *SearchHandler {
	return &SearchHandler{
		service: svc,
	}
}

func (h *SearchHandler) SearchMerchants(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]string{"message": "SearchMerchants endpoint"})
}

func (h *SearchHandler) SearchMenuItems(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]string{"message": "SearchMenuItems endpoint"})
}

func (h *SearchHandler) GetTrending(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]string{"message": "GetTrending endpoint"})
}

func (h *SearchHandler) SearchSuggest(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]string{"message": "SearchSuggest endpoint"})
}

func (h *SearchHandler) GetPopular(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]string{"message": "GetPopular endpoint"})
}

func (h *SearchHandler) IndexMerchant(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]string{"message": "IndexMerchant endpoint"})
}

func (h *SearchHandler) DeleteMerchantIndex(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]string{"message": "DeleteMerchantIndex endpoint"})
}

func (h *SearchHandler) IndexMenuItem(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]string{"message": "IndexMenuItem endpoint"})
}

func (h *SearchHandler) DeleteMenuItemIndex(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]string{"message": "DeleteMenuItemIndex endpoint"})
}

func (h *SearchHandler) TriggerReindex(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusAccepted, map[string]string{"message": "Reindex job started"})
}
