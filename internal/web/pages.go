package web

import (
	"io/fs"
	"net/http"
	"strings"
)

func serveAsset(response http.ResponseWriter, path, contentType string) {
	content, err := fs.ReadFile(assets, path)
	if err != nil {
		http.Error(response, "资源不存在", http.StatusNotFound)
		return
	}
	response.Header().Set("Content-Type", contentType)
	response.Header().Set("Cache-Control", "no-cache")
	_, _ = response.Write(content)
}

func (h *Handler) HandleQueuePage(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}
	serveAsset(response, "templates/index.html", "text/html; charset=utf-8")
}

func (h *Handler) HandleNewCasePage(response http.ResponseWriter, _ *http.Request) {
	serveAsset(response, "templates/new.html", "text/html; charset=utf-8")
}

func (h *Handler) HandleCasePage(response http.ResponseWriter, request *http.Request) {
	if strings.TrimSpace(request.PathValue("id")) == "" {
		http.NotFound(response, request)
		return
	}
	serveAsset(response, "templates/case.html", "text/html; charset=utf-8")
}

func (h *Handler) HandleAuthorPage(response http.ResponseWriter, request *http.Request) {
	if strings.TrimSpace(request.PathValue("id")) == "" {
		http.NotFound(response, request)
		return
	}
	serveAsset(response, "templates/author.html", "text/html; charset=utf-8")
}

func (h *Handler) HandleStyles(response http.ResponseWriter, _ *http.Request) {
	serveAsset(response, "static/app.css", "text/css; charset=utf-8")
}

func (h *Handler) HandleScript(response http.ResponseWriter, _ *http.Request) {
	serveAsset(response, "static/app.js", "text/javascript; charset=utf-8")
}
