package web

import (
	"net/http"

	"image-integrity-review/internal/application"
)

func (h *Handler) HandleAuthorCase(response http.ResponseWriter, request *http.Request) {
	item, err := h.service.GetAuthorCase(request.Context(), request.PathValue("id"), accessToken(request))
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item})
}

func (h *Handler) HandleAuthorResponse(response http.ResponseWriter, request *http.Request) {
	var input application.AuthorResponseInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	if input.AccessToken == "" {
		input.AccessToken = accessToken(request)
	}
	item, replayed, err := h.service.SubmitAuthorResponse(request.Context(), request.PathValue("id"), input)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item, "replayed": replayed})
}

func (h *Handler) HandleAuthorComplete(response http.ResponseWriter, request *http.Request) {
	var input struct {
		application.WriteContext
		AccessToken string `json:"access_token"`
		RoundNumber int    `json:"round_number"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	if input.AccessToken == "" {
		input.AccessToken = accessToken(request)
	}
	item, replayed, err := h.service.FinishAuthorResponseRound(request.Context(), request.PathValue("id"), input.AccessToken, input.RoundNumber, input.WriteContext)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item, "replayed": replayed})
}
