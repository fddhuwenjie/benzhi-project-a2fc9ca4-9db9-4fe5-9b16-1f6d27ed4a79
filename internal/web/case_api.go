package web

import (
	"net/http"
	"strconv"
	"strings"

	"image-integrity-review/internal/application"
	"image-integrity-review/internal/domain"
)

func (h *Handler) HandleHealth(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) HandleListCases(response http.ResponseWriter, request *http.Request) {
	filter, err := parseQueueFilter(request)
	if err != nil {
		writeError(response, err)
		return
	}
	view, err := h.service.QueueFiltered(request.Context(), filter)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, view)
}

func (h *Handler) HandleGetCase(response http.ResponseWriter, request *http.Request) {
	item, err := h.service.GetCase(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(response, err)
		return
	}
	payload := map[string]any{"case": item}
	if item.State == domain.StateDraft {
		preflight, preflightErr := h.service.DraftPreflight(request.Context(), item.ID)
		if preflightErr != nil {
			writeError(response, preflightErr)
			return
		}
		payload["preflight"] = preflight
	}
	writeJSON(response, http.StatusOK, payload)
}

func (h *Handler) HandleReviseDraftFigures(response http.ResponseWriter, request *http.Request) {
	var input application.ReviseFiguresInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	item, preflight, replayed, err := h.service.ReviseDraftFigures(request.Context(), request.PathValue("id"), input)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item, "preflight": preflight, "replayed": replayed})
}

func parseQueueFilter(request *http.Request) (application.QueueFilter, error) {
	query := request.URL.Query()
	allowed := map[string]bool{"status": true, "journal_section": true, "assignee_id": true, "q": true, "severity": true, "open_only": true, "page": true, "page_size": true}
	for key, values := range query {
		if !allowed[key] {
			return application.QueueFilter{}, domain.ValidationError(domain.FieldError{Field: key, Message: "未知查询参数"})
		}
		if len(values) != 1 {
			return application.QueueFilter{}, domain.ValidationError(domain.FieldError{Field: key, Message: "查询参数不能重复"})
		}
	}
	filter := application.QueueFilter{
		State: domain.CaseState(strings.TrimSpace(query.Get("status"))), JournalSection: strings.TrimSpace(query.Get("journal_section")),
		AssigneeID: strings.TrimSpace(query.Get("assignee_id")), Keyword: strings.TrimSpace(query.Get("q")),
		Severity: domain.Severity(strings.TrimSpace(query.Get("severity"))), Page: 1, PageSize: 20,
	}
	fields := make([]domain.FieldError, 0)
	if value := query.Get("page"); value != "" {
		page, err := strconv.Atoi(value)
		if err != nil {
			fields = append(fields, domain.FieldError{Field: "page", Message: "page 必须为整数"})
		} else {
			filter.Page = page
		}
	}
	if value := query.Get("page_size"); value != "" {
		pageSize, err := strconv.Atoi(value)
		if err != nil {
			fields = append(fields, domain.FieldError{Field: "page_size", Message: "page_size 必须为整数"})
		} else {
			filter.PageSize = pageSize
		}
	}
	if value := query.Get("open_only"); value != "" {
		openOnly, err := strconv.ParseBool(value)
		if err != nil {
			fields = append(fields, domain.FieldError{Field: "open_only", Message: "open_only 必须为 true 或 false"})
		} else {
			filter.OpenOnly = openOnly
		}
	}
	if len(fields) > 0 {
		return application.QueueFilter{}, domain.ValidationError(fields...)
	}
	return filter, nil
}

func (h *Handler) HandleCreateCase(response http.ResponseWriter, request *http.Request) {
	var input application.CreateCaseInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	item, replayed, err := h.service.CreateCase(request.Context(), input)
	if err != nil {
		writeError(response, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	writeJSON(response, status, map[string]any{"case": item, "replayed": replayed})
}

func (h *Handler) HandleSubmitDraft(response http.ResponseWriter, request *http.Request) {
	var input application.WriteContext
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	item, replayed, err := h.service.SubmitDraft(request.Context(), request.PathValue("id"), input)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item, "replayed": replayed})
}

func (h *Handler) HandleClaimCase(response http.ResponseWriter, request *http.Request) {
	var input application.WriteContext
	if err := decodeJSON(response, request, &input); err != nil {
		writeError(response, err)
		return
	}
	item, replayed, err := h.service.ClaimCase(request.Context(), request.PathValue("id"), input)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"case": item, "replayed": replayed})
}
