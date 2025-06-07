package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

// NewRouter creates a new HTTP router and registers API routes.
func NewRouter(apiHandler *APIHandler) *mux.Router {
	router := mux.NewRouter()

	// Funnel management endpoints
	router.HandleFunc("/api/funnel/enable", apiHandler.EnableFunnelHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/funnel/disable", apiHandler.DisableFunnelHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/funnel/extend", apiHandler.ExtendFunnelHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/funnel/status", apiHandler.GetFunnelStatusHandler).Methods(http.MethodGet)

	return router
}
