package web

import "net/http"

func (h *Handler) HandleTimeline(response http.ResponseWriter, request *http.Request) {
	events, err := h.service.Timeline(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"timeline": events})
}

func (h *Handler) HandleArchiveDownload(response http.ResponseWriter, request *http.Request) {
	document, err := h.service.DownloadArchive(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(response, err)
		return
	}
	response.Header().Set("Content-Disposition", "attachment; filename=review-archive-"+document.CaseID+".json")
	writeJSON(response, http.StatusOK, document)
}

func (h *Handler) HandleArchiveVerify(response http.ResponseWriter, request *http.Request) {
	var input struct {
		CaseReference string `json:"case_reference"`
		Digest        string `json:"digest"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	valid, err := h.service.VerifyArchive(request.Context(), input.CaseReference, input.Digest)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"valid": valid, "case_reference": input.CaseReference, "digest": input.Digest})
}
