package scim

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/smarzola/ldaplite/internal/directory"
	"github.com/smarzola/ldaplite/internal/store"
	"github.com/smarzola/ldaplite/pkg/config"
)

type Handler struct {
	store    store.Store
	cfg      *config.Config
	service  *directory.Service
	contract Contract
}

func NewHandler(st store.Store, cfg *config.Config) *Handler {
	return &Handler{
		store:    st,
		cfg:      cfg,
		service:  directory.NewService(st, cfg),
		contract: DefaultContract(),
	}
}

func (h *Handler) Contract() Contract {
	return h.contract
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writeSCIMError(w, http.StatusNotImplemented, "SCIM endpoint is not implemented yet")
}

type errorResponse struct {
	Schemas []string `json:"schemas"`
	Detail  string   `json:"detail"`
	Status  string   `json:"status"`
}

func writeSCIMError(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Schemas: []string{errorSchema},
		Detail:  detail,
		Status:  strconv.Itoa(status),
	})
}
