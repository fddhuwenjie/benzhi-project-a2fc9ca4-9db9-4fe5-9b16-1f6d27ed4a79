package web

import (
	"net/http"

	"image-integrity-review/internal/application"
)

func (h *Handler) HandleVerdict(response http.ResponseWriter, request *http.Request) {
	var input application.VerdictInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	item, replayed, err := h.service.RecordVerdict(request.Context(), request.PathValue("id"), input)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item, "replayed": replayed})
}

func (h *Handler) HandleBatchVerdicts(response http.ResponseWriter, request *http.Request) {
	var input application.BatchVerdictInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	item, replayed, err := h.service.RecordVerdicts(request.Context(), request.PathValue("id"), input)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item, "replayed": replayed})
}

func (h *Handler) HandleAuthorRequest(response http.ResponseWriter, request *http.Request) {
	var input application.WriteContext
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	credential, err := h.service.BeginAuthorResponse(request.Context(), request.PathValue("id"), input)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, credential)
}

func (h *Handler) HandleResolution(response http.ResponseWriter, request *http.Request) {
	var input application.ResolutionInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	item, replayed, err := h.service.ResolveFinding(request.Context(), request.PathValue("id"), input)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item, "replayed": replayed})
}

func (h *Handler) HandleRecheckComplete(response http.ResponseWriter, request *http.Request) {
	var input application.WriteContext
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	item, replayed, err := h.service.FinishRecheck(request.Context(), request.PathValue("id"), input)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item, "replayed": replayed})
}

func (h *Handler) HandleDecision(response http.ResponseWriter, request *http.Request) {
	var input application.DecisionInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	item, replayed, err := h.service.Decide(request.Context(), request.PathValue("id"), input)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item, "replayed": replayed})
}
