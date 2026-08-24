package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"image-integrity-review/internal/application"
	"image-integrity-review/internal/repository"
	"image-integrity-review/internal/web"
)

type config struct {
	addr      string
	dataDir   string
	selfCheck bool
}

func main() {
	settings := parseConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := validateAddress(settings.addr); err != nil {
		logger.Error("地址配置无效", "error", err)
		os.Exit(2)
	}
	if settings.selfCheck {
		if err := runSelfCheck(settings); err != nil {
			logger.Error("自检失败", "error", err)
			os.Exit(1)
		}
		logger.Info("自检完成")
		return
	}
	if err := serve(settings, logger); err != nil {
		logger.Error("服务退出", "error", err)
		os.Exit(1)
	}
}

func parseConfig() config {
	defaultAddr := "127.0.0.1:19081"
	addr := flag.String("addr", envAddress(defaultAddr), "HTTP 监听地址，例如 127.0.0.1:19081")
	dataDir := flag.String("data-dir", defaultDataDir(), "本地 JSONL 与快照目录")
	selfCheck := flag.Bool("self-check", false, "运行有界端到端自检后退出")
	flag.Parse()
	return config{addr: *addr, dataDir: *dataDir, selfCheck: *selfCheck}
}

func envAddress(fallback string) string {
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return "127.0.0.1:" + port
	}
	return fallback
}

func defaultDataDir() string {
	if value := strings.TrimSpace(os.Getenv("IMAGE_REVIEW_DATA")); value != "" {
		return value
	}
	return filepath.Join(os.TempDir(), "image-integrity-review")
}

func validateAddress(addr string) error {
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		return fmt.Errorf("服务只允许绑定 127.0.0.1 回环地址，当前为 %s", addr)
	}
	return nil
}

func buildServer(settings config) (*http.Server, *application.Service, error) {
	store, err := repository.OpenFileStore(settings.dataDir)
	if err != nil {
		return nil, nil, err
	}
	service := application.NewService(store)
	handler := web.NewHandler(service)
	server := &http.Server{
		Addr:              settings.addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       45 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	return server, service, nil
}

func serve(settings config, logger *slog.Logger) error {
	server, _, err := buildServer(settings)
	if err != nil {
		return err
	}
	stop, stopCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopCancel()
	go func() {
		<-stop.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	logger.Info("图像完整性审查服务启动", "addr", server.Addr, "data_dir", settings.dataDir)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
