package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func runSelfCheck(settings config) error {
	dataDir, err := os.MkdirTemp("", "image-review-self-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)
	settings.dataDir = filepath.Join(dataDir, "store")
	server, _, err := buildServer(settings)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", settings.addr)
	if err != nil {
		return fmt.Errorf("监听自检地址: %w", err)
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	client := &http.Client{Timeout: 3 * time.Second}
	baseURL := "http://" + settings.addr
	shutdown := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
	defer shutdown()
	if err := selfCheckFlow(client, baseURL); err != nil {
		return err
	}
	select {
	case err := <-serveErrors:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	default:
	}
	return nil
}

func selfCheckFlow(client *http.Client, baseURL string) error {
	digest := func(char byte) string { return string(bytes.Repeat([]byte{char}, 32)) }
	createPayload := map[string]any{
		"actor": map[string]string{"type": "editor", "id": "editor-self-check"}, "request_id": "self-create", "expected_revision": 0,
		"manuscript_code": "SELF-CHECK-001", "title": "图像完整性自检稿件", "journal_section": "方法学",
		"figures": []map[string]any{{"figure_label": "Figure 1", "panel_label": "A", "caption": "自检图像", "content_digest": digest('a'), "pixel_width": 1200, "pixel_height": 900, "experiment_source": "实验记录 S-1", "raw_data_reference": "raw://self-check/1"}, {"figure_label": "Figure 1", "panel_label": "B", "caption": "重复摘要用于触发规则", "content_digest": digest('a'), "pixel_width": 1200, "pixel_height": 900, "experiment_source": "实验记录 S-1", "raw_data_reference": "raw://self-check/2"}},
	}
	created, err := doJSON(client, http.MethodPost, baseURL+"/api/cases", createPayload)
	if err != nil {
		return fmt.Errorf("创建案件: %w", err)
	}
	item := created["case"].(map[string]any)
	caseID := item["id"].(string)
	revision := int64(item["revision"].(float64))
	figures := item["figures"].([]any)
	figures[0].(map[string]any)["experiment_source"] = "实验记录 S-1（已核对）"
	revised, err := doJSON(client, http.MethodPut, baseURL+"/api/cases/"+caseID+"/figures", map[string]any{
		"actor": map[string]string{"type": "editor", "id": "editor-self-check"}, "request_id": "self-revise-figures", "expected_revision": revision, "figures": figures,
	})
	if err != nil {
		return fmt.Errorf("修订草稿图像: %w", err)
	}
	revision = int64(revised["case"].(map[string]any)["revision"].(float64))
	write := func(path string, payload map[string]any) (map[string]any, error) {
		payload["expected_revision"] = revision
		result, err := doJSON(client, http.MethodPost, baseURL+path, payload)
		if err == nil {
			revision = int64(result["case"].(map[string]any)["revision"].(float64))
		}
		return result, err
	}
	if _, err := write("/api/cases/"+caseID+"/submit", map[string]any{"actor": map[string]string{"type": "editor", "id": "editor-self-check"}, "request_id": "self-submit"}); err != nil {
		return err
	}
	if _, err := doJSON(client, http.MethodGet, baseURL+"/api/cases?status=pending_review&severity=critical&open_only=true&page=1&page_size=10", nil); err != nil {
		return fmt.Errorf("筛选案件队列: %w", err)
	}
	if _, err := write("/api/cases/"+caseID+"/claim", map[string]any{"actor": map[string]string{"type": "reviewer", "id": "reviewer-self-check"}, "request_id": "self-claim"}); err != nil {
		return err
	}
	caseData, err := doJSON(client, http.MethodGet, baseURL+"/api/cases/"+caseID, nil)
	if err != nil {
		return err
	}
	findings := caseData["case"].(map[string]any)["findings"].([]any)
	batchItems := make([]map[string]any, 0, len(findings))
	for _, raw := range findings {
		finding := raw.(map[string]any)
		batchItems = append(batchItems, map[string]any{"finding_id": finding["id"], "verdict": "needs_explanation", "note": "自检要求作者补充"})
	}
	if _, err := write("/api/cases/"+caseID+"/verdicts/batch", map[string]any{"actor": map[string]string{"type": "reviewer", "id": "reviewer-self-check"}, "request_id": "self-batch-verdict", "items": batchItems}); err != nil {
		return err
	}
	credential, err := write("/api/cases/"+caseID+"/author-request", map[string]any{"actor": map[string]string{"type": "reviewer", "id": "reviewer-self-check"}, "request_id": "self-author-request"})
	if err != nil {
		return err
	}
	token := credential["access_token"].(string)
	roundNumber := int64(credential["case"].(map[string]any)["current_round"].(float64))
	caseData, err = doJSON(client, http.MethodGet, baseURL+"/api/author/cases/"+caseID+"?access_token="+token, nil)
	if err != nil {
		return err
	}
	findings = caseData["case"].(map[string]any)["findings"].([]any)
	for index, raw := range findings {
		finding := raw.(map[string]any)
		if _, err := write("/api/author/cases/"+caseID+"/responses", map[string]any{"actor": map[string]string{"type": "author", "id": "author-self-check"}, "request_id": fmt.Sprintf("self-response-%d", index), "access_token": token, "round_number": roundNumber, "finding_id": finding["id"], "explanation": "自检作者说明：已核对原始数据并解释图像处理步骤。", "replacement_digest": digest(byte('b' + byte(index))), "raw_data_reference": "raw://self-check/verified"}); err != nil {
			return err
		}
	}
	if _, err := write("/api/author/cases/"+caseID+"/complete", map[string]any{"actor": map[string]string{"type": "author", "id": "author-self-check"}, "request_id": "self-author-complete", "access_token": token, "round_number": roundNumber}); err != nil {
		return err
	}
	caseData, err = doJSON(client, http.MethodGet, baseURL+"/api/cases/"+caseID, nil)
	if err != nil {
		return err
	}
	findings = caseData["case"].(map[string]any)["findings"].([]any)
	for index, raw := range findings {
		finding := raw.(map[string]any)
		if _, err := write("/api/cases/"+caseID+"/resolutions", map[string]any{"actor": map[string]string{"type": "reviewer", "id": "reviewer-self-check"}, "request_id": fmt.Sprintf("self-resolution-%d", index), "round_number": roundNumber, "finding_id": finding["id"], "resolution": "accepted", "note": "自检复核通过"}); err != nil {
			return err
		}
	}
	if _, err := write("/api/cases/"+caseID+"/recheck-complete", map[string]any{"actor": map[string]string{"type": "reviewer", "id": "reviewer-self-check"}, "request_id": "self-recheck-complete"}); err != nil {
		return err
	}
	approved, err := write("/api/cases/"+caseID+"/decision", map[string]any{"actor": map[string]string{"type": "editor", "id": "editor-self-check"}, "request_id": "self-decision", "decision": "approved", "note": "自检终审通过"})
	if err != nil {
		return err
	}
	archiveDigest := approved["case"].(map[string]any)["archive_digest"].(string)
	verify, err := doJSON(client, http.MethodPost, baseURL+"/api/archives/verify", map[string]any{"case_reference": "SELF-CHECK-001", "digest": archiveDigest})
	if err != nil {
		return err
	}
	if valid, ok := verify["valid"].(bool); !ok || !valid {
		return fmt.Errorf("归档摘要校验未通过")
	}
	if _, err := doJSON(client, http.MethodGet, baseURL+"/api/health", nil); err != nil {
		return err
	}
	return nil
}

func doJSON(client *http.Client, method, url string, value any) (map[string]any, error) {
	var body io.Reader
	if value != nil {
		content, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(content)
	}
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if value != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, fmt.Errorf("解析 %s 响应: %w", url, err)
	}
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("%s 返回 %d: %v", url, response.StatusCode, payload["error"])
	}
	return payload, nil
}
