package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"openwatcher/internal/config"
	"openwatcher/internal/quota"
	"openwatcher/internal/server"
	"openwatcher/internal/sessions"
)

const widgetEndpointLinePrefix = "OPENWATCHER_WIDGET_ENDPOINT="

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
	var widgetListen string

	flag.StringVar(&configPath, "config", "", "config file path")
	flag.StringVar(&listen, "listen", "", "listen address, for example 127.0.0.1:8787")
	flag.StringVar(&publicBaseURL, "public-base-url", "", "public base URL for watches and Desktop bootstrap")
	flag.StringVar(&pairingSlot, "pairing-slot", string(config.PairingSlotBeta), "pairing slot: beta or dev")
	flag.BoolVar(&pairMode, "pair", false, "temporarily allow replacing the paired watch token")
	flag.BoolVar(&noAuth, "no-auth", false, "disable watcher token auth; development only")
	flag.StringVar(&widgetListen, "widget-listen", "", "loopback address for the read-only desktop widget, for example 127.0.0.1:0")

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
	var widgetServer *http.Server
	if strings.TrimSpace(widgetListen) != "" {
		if !validWidgetTokenHash(cfg.WidgetTokenHash) {
			log.Fatal("悬浮球监听已配置，但 widgetTokenHash 无效")
		}
		var widgetListener net.Listener
		widgetServer, widgetListener, err = startWidgetServer(app, widgetListen)
		if err != nil {
			log.Fatalf("悬浮球监听启动失败: %v", err)
		}
		fmt.Fprintln(os.Stdout, formatWidgetEndpointLine(widgetListener.Addr()))
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if widgetServer != nil {
			if err := widgetServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("悬浮球服务关闭失败: %v", err)
			}
		}
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

func startWidgetServer(app *server.App, address string) (*http.Server, net.Listener, error) {
	if err := validateWidgetListenAddress(address); err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, nil, err
	}
	httpServer := &http.Server{
		Handler:           app.WidgetHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("悬浮球服务异常退出: %v", err)
		}
	}()
	return httpServer, listener, nil
}

func validateWidgetListenAddress(address string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return fmt.Errorf("必须是 loopback host:port")
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("只允许 loopback 地址")
	}
	if strings.TrimSpace(port) == "" {
		return fmt.Errorf("端口不能为空")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 0 || portNumber > 65535 {
		return fmt.Errorf("端口无效")
	}
	return nil
}

func validWidgetTokenHash(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size
}

func formatWidgetEndpointLine(address net.Addr) string {
	return widgetEndpointLinePrefix + "http://" + address.String()
}

func parseWidgetEndpointLine(line string) (string, bool) {
	endpoint := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), widgetEndpointLinePrefix))
	if endpoint == "" || !strings.HasPrefix(endpoint, "http://") {
		return "", false
	}
	return endpoint, true
}

func normalizeArgsForServeAlias(args []string) []string {
	if len(args) > 1 && args[1] == "serve" {
		return append([]string{args[0]}, args[2:]...)
	}
	return args
}
