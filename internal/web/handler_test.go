package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"image-integrity-review/internal/application"
	"image-integrity-review/internal/repository"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := repository.OpenFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(application.NewService(store)).Routes()
}

func TestPagesAndSecurityHeaders(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/cases/new", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "<!doctype html>") || !strings.Contains(response.Body.String(), "<body") {
		t.Fatal("页面缺少完整 HTML 结构")
	}
	if response.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("缺少安全响应头")
	}
}

func TestAPIRejectsUnknownJSONFields(t *testing.T) {
	handler := testHandler(t)
	body := `{"actor":{"type":"editor","id":"e"},"request_id":"r","expected_revision":0,"manuscript_code":"M","title":"T","journal_section":"S","figures":[],"unexpected":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d，响应 %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "validation_failed") {
		t.Fatalf("错误格式无效：%s", response.Body.String())
	}
}

func TestQueueRejectsInvalidFilters(t *testing.T) {
	handler := testHandler(t)
	for _, path := range []string{"/api/cases?status=unknown", "/api/cases?page=0", "/api/cases?page_size=101", "/api/cases?severity=urgent"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "fields") {
			t.Fatalf("%s 应返回字段错误，实际 %d %s", path, response.Code, response.Body.String())
		}
	}
}

func TestDraftFigureRevisionAPIIsAtomicAndIdempotent(t *testing.T) {
	handler := testHandler(t)
	createBody := `{"actor":{"type":"editor","id":"editor-1"},"request_id":"create-draft","expected_revision":0,"manuscript_code":"API-001","title":"接口修订测试","journal_section":"研究论文","figures":[{"figure_label":"Figure 1","panel_label":"A","caption":"原图","content_digest":"aaaaaaaaaaaaaaaa","pixel_width":100,"pixel_height":100,"experiment_source":"来源一","raw_data_reference":"raw://1"}]}`
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(createBody)))
	if created.Code != http.StatusCreated {
		t.Fatalf("创建失败：%d %s", created.Code, created.Body.String())
	}
	var createdPayload struct {
		Case struct {
			ID       string `json:"id"`
			Revision int64  `json:"revision"`
			Figures  []struct {
				ID string `json:"id"`
			} `json:"figures"`
		} `json:"case"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdPayload); err != nil {
		t.Fatal(err)
	}
	caseID := createdPayload.Case.ID
	figureID := createdPayload.Case.Figures[0].ID
	invalidBody := `{"actor":{"type":"editor","id":"editor-1"},"request_id":"invalid-revise","expected_revision":1,"figures":[{"id":"` + figureID + `","figure_label":"Figure 1","panel_label":"A","caption":"原图","content_digest":"bad","pixel_width":100,"pixel_height":100}]}`
	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodPut, "/api/cases/"+caseID+"/figures", strings.NewReader(invalidBody)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("无效修订应失败：%d %s", invalid.Code, invalid.Body.String())
	}
	validBody := `{"actor":{"type":"editor","id":"editor-1"},"request_id":"valid-revise","expected_revision":1,"figures":[{"id":"` + figureID + `","figure_label":"Figure 1","panel_label":"A","caption":"已修订","content_digest":"aaaaaaaaaaaaaaaa","pixel_width":100,"pixel_height":100,"experiment_source":"来源二"},{"figure_label":"Figure 2","caption":"新增图","content_digest":"bbbbbbbbbbbbbbbb","pixel_width":200,"pixel_height":200}]}`
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodPut, "/api/cases/"+caseID+"/figures", strings.NewReader(validBody)))
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"revision":2`) {
		t.Fatalf("有效修订失败：%d %s", first.Code, first.Body.String())
	}
	replay := httptest.NewRecorder()
	handler.ServeHTTP(replay, httptest.NewRequest(http.MethodPut, "/api/cases/"+caseID+"/figures", strings.NewReader(validBody)))
	if replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), `"replayed":true`) || !strings.Contains(replay.Body.String(), `"revision":2`) {
		t.Fatalf("幂等重放失败：%d %s", replay.Code, replay.Body.String())
	}
}
