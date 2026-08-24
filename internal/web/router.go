package web

import (
	"net/http"

	"image-integrity-review/internal/application"
)

type Handler struct {
	service *application.Service
}

func NewHandler(service *application.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.HandleQueuePage)
	mux.HandleFunc("GET /cases/new", h.HandleNewCasePage)
	mux.HandleFunc("GET /cases/{id}", h.HandleCasePage)
	mux.HandleFunc("GET /author/{id}", h.HandleAuthorPage)
	mux.HandleFunc("GET /assets/app.css", h.HandleStyles)
	mux.HandleFunc("GET /assets/app.js", h.HandleScript)
	mux.HandleFunc("GET /api/health", h.HandleHealth)
	mux.HandleFunc("GET /api/cases", h.HandleListCases)
	mux.HandleFunc("POST /api/cases", h.HandleCreateCase)
	mux.HandleFunc("GET /api/cases/{id}", h.HandleGetCase)
	mux.HandleFunc("PUT /api/cases/{id}/figures", h.HandleReviseDraftFigures)
	mux.HandleFunc("POST /api/cases/{id}/submit", h.HandleSubmitDraft)
	mux.HandleFunc("POST /api/cases/{id}/claim", h.HandleClaimCase)
	mux.HandleFunc("POST /api/cases/{id}/verdicts", h.HandleVerdict)
	mux.HandleFunc("POST /api/cases/{id}/verdicts/batch", h.HandleBatchVerdicts)
	mux.HandleFunc("POST /api/cases/{id}/author-request", h.HandleAuthorRequest)
	mux.HandleFunc("GET /api/author/cases/{id}", h.HandleAuthorCase)
	mux.HandleFunc("POST /api/author/cases/{id}/responses", h.HandleAuthorResponse)
	mux.HandleFunc("POST /api/author/cases/{id}/complete", h.HandleAuthorComplete)
	mux.HandleFunc("POST /api/cases/{id}/resolutions", h.HandleResolution)
	mux.HandleFunc("POST /api/cases/{id}/recheck-complete", h.HandleRecheckComplete)
	mux.HandleFunc("POST /api/cases/{id}/decision", h.HandleDecision)
	mux.HandleFunc("GET /api/cases/{id}/timeline", h.HandleTimeline)
	mux.HandleFunc("GET /api/cases/{id}/archive", h.HandleArchiveDownload)
	mux.HandleFunc("POST /api/archives/verify", h.HandleArchiveVerify)
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("X-Frame-Options", "DENY")
		response.Header().Set("Referrer-Policy", "same-origin")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(response, request)
	})
}
