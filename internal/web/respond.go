package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"image-integrity-review/internal/domain"
	"image-integrity-review/internal/repository"
)

const maxRequestBody = 1 << 20

type apiError struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Fields  []domain.FieldError `json:"fields,omitempty"`
}

func decodeJSON(response http.ResponseWriter, request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBody)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.ValidationError(domain.FieldError{Field: "body", Message: "JSON 请求体无效：" + err.Error()})
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.ValidationError(domain.FieldError{Field: "body", Message: "请求体只能包含一个 JSON 对象"})
	}
	return nil
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeError(response http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	body := errorBody{Code: "internal_error", Message: "服务暂时无法处理请求"}
	var domainErr *domain.DomainError
	if errors.As(err, &domainErr) {
		body.Code = string(domainErr.Code)
		body.Message = domainErr.Message
		body.Fields = domainErr.Fields
		switch domainErr.Code {
		case domain.CodeValidation:
			status = http.StatusBadRequest
		case domain.CodeNotFound:
			status = http.StatusNotFound
		case domain.CodeForbidden:
			status = http.StatusForbidden
		case domain.CodeConflict, domain.CodeState, domain.CodeFrozen:
			status = http.StatusConflict
		}
	} else if errors.Is(err, repository.ErrNotFound) {
		status = http.StatusNotFound
		body = errorBody{Code: "not_found", Message: "案件不存在"}
	} else if errors.Is(err, repository.ErrRevision) {
		status = http.StatusConflict
		body = errorBody{Code: "revision_conflict", Message: err.Error()}
	}
	writeJSON(response, status, apiError{Error: body})
}

func accessToken(request *http.Request) string {
	if token := strings.TrimSpace(request.Header.Get("X-Author-Access")); token != "" {
		return token
	}
	return strings.TrimSpace(request.URL.Query().Get("access_token"))
}
