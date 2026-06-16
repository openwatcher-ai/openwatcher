package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"openwatcher/internal/config"
	"openwatcher/internal/quota"
	"openwatcher/internal/server"
	"openwatcher/internal/sessions"
)

func main() {
	if handled, exitCode := maybeRunCodexCompactHook(os.Args); handled {
		os.Exit(exitCode)
	}
	if handled, exitCode := newDevAllowlistCommand().maybeRun(os.Args); handled {
		os.Exit(exitCode)
	}

	var configPath string
	var listen string
	var publicBaseURL string
	var pairingSlot string
	var pairMode bool
	var noAuth bool

	flag.StringVar(&configPath, "config", "", "config file path")
	flag.StringVar(&listen, "listen", "", "listen address, for example 127.0.0.1:8787")
	flag.StringVar(&publicBaseURL, "public-base-url", "", "public base URL for watches and Desktop bootstrap")
	flag.StringVar(&pairingSlot, "pairing-slot", string(config.PairingSlotBeta), "pairing slot: beta or dev")
	flag.BoolVar(&pairMode, "pair", false, "temporarily allow replacing the paired watch token")
	flag.BoolVar(&noAuth, "no-auth", false, "disable watcher token auth; development only")

	os.Args = normalizeArgsForServeAlias(os.Args)
	flag.Parse()

	cfg, resolvedConfigPath, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("读取配置失败: %v", err)
	}
	if listen != "" {
		cfg.Listen = listen
	}
	if publicBaseURL != "" {
		cfg.PublicBaseURL = publicBaseURL
	}
	cfg.ApplyDefaults()
	selectedSlot := config.NormalizePairingSlot(pairingSlot)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	quotaClient := quota.NewClient(cfg.CodexHome)
	quotaRefresher := quota.NewRefresher(quotaClient, time.Duration(cfg.QuotaRefreshSeconds)*time.Second)
	quotaRefresher.Start(ctx)

	sessionScanner := sessions.NewScanner(
		cfg.CodexHome,
		cfg.ActiveSessionLimit,
	)

	pairingAllowed := pairMode || cfg.TokenHashForSlot(selectedSlot) == ""
	app := server.New(resolvedConfigPath, cfg, pairingAllowed, quotaRefresher, sessionScanner)
	app.SetPairingSlot(selectedSlot)
	app.SetNoAuth(noAuth)

	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("服务关闭失败: %v", err)
		}
	}()

	if noAuth && pairingAllowed {
		log.Printf("服务以 no-auth 和配对等待状态启动，监听 %s，配置文件 %s", cfg.Listen, resolvedConfigPath)
	} else if noAuth {
		log.Printf("服务以 no-auth 模式启动，监听 %s，配置文件 %s", cfg.Listen, resolvedConfigPath)
	} else if pairingAllowed {
		log.Printf("服务已进入配对等待状态，监听 %s，配置文件 %s", cfg.Listen, resolvedConfigPath)
	} else {
		log.Printf("服务启动，监听 %s", cfg.Listen)
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务启动失败: %v", err)
	}

	fmt.Fprintln(os.Stderr, "服务已停止")
}

func normalizeArgsForServeAlias(args []string) []string {
	if len(args) > 1 && args[1] == "serve" {
		return append([]string{args[0]}, args[2:]...)
	}
	return args
}
