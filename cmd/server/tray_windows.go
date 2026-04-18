//go:build windows

package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/browser"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/cmd"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

const (
	trayDefaultPort = 8317
	trayStopTimeout = 30 * time.Second
)

const (
	swHide = 0
)

var (
	kernel32DLL          = windows.NewLazySystemDLL("kernel32.dll")
	getConsoleWindowProc = kernel32DLL.NewProc("GetConsoleWindow")
	freeConsoleProc      = kernel32DLL.NewProc("FreeConsole")
	user32DLL            = windows.NewLazySystemDLL("user32.dll")
	showWindowProc       = user32DLL.NewProc("ShowWindow")
	detachConsoleOnce    sync.Once
)

//go:embed app.ico
var embeddedTrayIcon []byte

type trayServiceController struct {
	mu      sync.Mutex
	port    int
	startFn func() *cmd.BackgroundServiceHandle
	handle  *cmd.BackgroundServiceHandle
}

func newTrayServiceController(cfg *config.Config, configPath, password string, port int) *trayServiceController {
	startConfigPath := strings.TrimSpace(configPath)
	return &trayServiceController{
		port: port,
		startFn: func() *cmd.BackgroundServiceHandle {
			runtimeCfg, err := loadTrayServiceConfig(cfg, startConfigPath)
			if err != nil {
				log.WithError(err).Warn("failed to reload config for tray service start; using last known config")
				runtimeCfg = cloneTrayConfig(cfg)
			}
			return cmd.StartServiceBackgroundHandle(runtimeCfg, startConfigPath, password)
		},
	}
}

func loadTrayServiceConfig(fallbackCfg *config.Config, configPath string) (*config.Config, error) {
	if strings.TrimSpace(configPath) == "" {
		return cloneTrayConfig(fallbackCfg), nil
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config for tray restart: %w", err)
	}
	if cfg == nil {
		return cloneTrayConfig(fallbackCfg), nil
	}

	resolvedAuthDir, err := util.ResolveAuthDir(cfg.AuthDir)
	if err != nil {
		return nil, fmt.Errorf("resolve auth directory for tray restart: %w", err)
	}
	cfg.AuthDir = resolvedAuthDir
	cfg.SyncAuthPoolFromAuthDir()
	usage.SetStatisticsEnabled(cfg.UsageStatisticsEnabled)
	coreauth.SetQuotaCooldownDisabled(cfg.DisableCooling)
	return cfg, nil
}

func cloneTrayConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return &config.Config{}
	}
	copyCfg := *cfg
	return &copyCfg
}

func (c *trayServiceController) ensureRunning() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.syncStateLocked()
	if c.handle != nil && isPortListening(c.port) {
		return nil
	}
	if c.handle != nil {
		if err := c.stopLocked(trayStopTimeout); err != nil {
			return err
		}
	}
	return c.startLocked()
}

func (c *trayServiceController) stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.syncStateLocked()
	return c.stopLocked(trayStopTimeout)
}

func (c *trayServiceController) restart() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.syncStateLocked()
	if err := c.stopLocked(trayStopTimeout); err != nil {
		return err
	}
	return c.startLocked()
}

func (c *trayServiceController) startLocked() error {
	handle := c.startFn()
	c.handle = handle

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if c.doneClosedLocked() {
			c.handle = nil
			return errors.New("service exited during startup")
		}
		if isPortListening(c.port) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	if c.doneClosedLocked() {
		c.handle = nil
		return errors.New("service exited before listening")
	}

	return fmt.Errorf("service did not listen on port %d within startup window", c.port)
}

func (c *trayServiceController) stopLocked(timeout time.Duration) error {
	if c.handle == nil {
		return nil
	}

	var stopErr error
	if err := c.handle.PersistUsageSnapshot(); err != nil {
		stopErr = err
		log.WithError(err).Warn("failed to persist usage snapshot before tray stop")
	}

	done := c.handle.Done()
	c.handle.Cancel()

	if done != nil {
		select {
		case <-done:
		case <-time.After(timeout):
			stopErr = errors.Join(stopErr, fmt.Errorf("service stop timeout reached after %s", timeout))
		}
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isPortListening(c.port) {
			c.handle = nil
			return stopErr
		}
		time.Sleep(250 * time.Millisecond)
	}

	if isPortListening(c.port) {
		stopErr = errors.Join(stopErr, fmt.Errorf("service is still listening on port %d after shutdown", c.port))
	}
	c.handle = nil
	return stopErr
}

func (c *trayServiceController) syncStateLocked() {
	if c.handle == nil {
		return
	}
	done := c.handle.Done()
	if done == nil {
		c.handle = nil
		return
	}
	select {
	case <-done:
		c.handle = nil
	default:
	}
}

func (c *trayServiceController) doneClosedLocked() bool {
	if c.handle == nil {
		return true
	}
	done := c.handle.Done()
	if done == nil {
		return true
	}
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func isPortListening(port int) bool {
	if port <= 0 {
		return false
	}

	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
	if err == nil {
		if errClose := conn.Close(); errClose != nil {
			log.WithError(errClose).Debug("failed to close probe connection")
		}
		return true
	}

	fallback := fmt.Sprintf("localhost:%d", port)
	conn, err = net.DialTimeout("tcp", fallback, 500*time.Millisecond)
	if err == nil {
		if errClose := conn.Close(); errClose != nil {
			log.WithError(errClose).Debug("failed to close fallback probe connection")
		}
		return true
	}
	return false
}

func shouldAutoTrayLaunch(args []string) bool {
	return len(args) == 0 && launchedFromExplorerProcess()
}

func shouldHideTrayConsole(trayMode bool, autoTrayLaunch bool) bool {
	if autoTrayLaunch {
		return true
	}
	return trayMode && launchedFromExplorerProcess()
}

func hideConsoleWindow() {
	hwnd, _, _ := getConsoleWindowProc.Call()
	if hwnd == 0 {
		return
	}
	_, _, _ = showWindowProc.Call(hwnd, swHide)
}

func detachConsoleWindow() {
	detachConsoleOnce.Do(func() {
		hideConsoleWindow()
		_, _, _ = freeConsoleProc.Call()

		devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			log.WithError(err).Debug("failed to redirect detached console streams")
			log.SetOutput(io.Discard)
			return
		}

		os.Stdout = devNull
		os.Stderr = devNull
		log.SetOutput(devNull)
	})
}

func launchedFromExplorerProcess() bool {
	parentPID := os.Getppid()
	if parentPID <= 0 {
		return false
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(parentPID))
	if err != nil {
		return false
	}
	defer func() {
		_ = windows.CloseHandle(handle)
	}()

	size := uint32(windows.MAX_PATH)
	buffer := make([]uint16, size)
	if errQuery := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); errQuery != nil {
		return false
	}

	name := strings.ToLower(filepath.Base(windows.UTF16ToString(buffer[:size])))
	return name == "explorer.exe"
}

func runTrayMode(cfg *config.Config, configPath, password string, openManagementOnLaunch bool) error {
	port := cfg.Port
	if port <= 0 {
		port = trayDefaultPort
	}

	mutexName := fmt.Sprintf("Global\\CPA_Tray_Icon_%d", port)
	mutexHandle, alreadyExists, err := acquireTrayMutex(mutexName)
	if err != nil {
		return fmt.Errorf("failed to acquire tray mutex: %w", err)
	}
	defer func() {
		if errClose := windows.CloseHandle(mutexHandle); errClose != nil {
			log.WithError(errClose).Warn("failed to close tray mutex handle")
		}
	}()

	if alreadyExists {
		log.Infof("tray mode is already running for port %d", port)
		if openManagementOnLaunch {
			if errOpen := openManagementPage(port); errOpen != nil {
				log.WithError(errOpen).Warn("failed to open management page for existing tray instance")
			}
		}
		return nil
	}

	controller := newTrayServiceController(cfg, configPath, password, port)

	watchdogStop := make(chan struct{})
	watchdogStopOnce := sync.Once{}
	stopWatchdog := func() {
		watchdogStopOnce.Do(func() {
			close(watchdogStop)
		})
	}

	onReady := func() {
		tooltip := fmt.Sprintf("CPA %d", port)
		systray.SetTitle(tooltip)
		systray.SetTooltip("CPA 服务正在运行")

		if icon := loadTrayIcon(); len(icon) > 0 {
			systray.SetIcon(icon)
		}

		openItem := systray.AddMenuItem("打开管理面板", "打开 CPA 管理面板")
		restartItem := systray.AddMenuItem("重启 CPA 服务", "重启后台 CPA 服务")
		systray.AddSeparator()
		exitAllItem := systray.AddMenuItem("彻底关闭 CPA 服务", "停止服务并退出系统托盘")

		openManagementWithEnsure := func() {
			if errEnsure := controller.ensureRunning(); errEnsure != nil {
				log.WithError(errEnsure).Warn("failed to ensure service before opening management page")
			}
			if errOpen := openManagementPage(port); errOpen != nil {
				log.WithError(errOpen).Warn("failed to open management page")
			}
		}
		systray.SetTrayClickHandler(openManagementWithEnsure)

		go func() {
			for {
				select {
				case <-openItem.ClickedCh:
					openManagementWithEnsure()
				case <-restartItem.ClickedCh:
					if errRestart := controller.restart(); errRestart != nil {
						log.WithError(errRestart).Warn("failed to restart service from tray")
						continue
					}
					openManagementWithEnsure()
				case <-exitAllItem.ClickedCh:
					if errStop := controller.stop(); errStop != nil {
						log.WithError(errStop).Warn("failed to stop service from tray exit")
					}
					systray.Quit()
					return
				}
			}
		}()

		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if errEnsure := controller.ensureRunning(); errEnsure != nil {
						log.WithError(errEnsure).Warn("tray watchdog failed to ensure running service")
					}
				case <-watchdogStop:
					return
				}
			}
		}()

		if errEnsure := controller.ensureRunning(); errEnsure != nil {
			log.WithError(errEnsure).Warn("failed to start service in tray mode")
		}
		if openManagementOnLaunch {
			openManagementWithEnsure()
		}
	}

	onExit := func() {
		stopWatchdog()
		systray.SetTrayClickHandler(nil)
		if errStop := controller.stop(); errStop != nil {
			log.WithError(errStop).Warn("failed to stop service during tray shutdown")
		}
	}

	systray.Run(onReady, onExit)
	return nil
}

func acquireTrayMutex(name string) (windows.Handle, bool, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, false, fmt.Errorf("invalid mutex name %q: %w", name, err)
	}

	handle, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil {
		return 0, false, err
	}

	alreadyExists := windows.GetLastError() == windows.ERROR_ALREADY_EXISTS
	return handle, alreadyExists, nil
}

func openManagementPage(port int) error {
	url := fmt.Sprintf("http://localhost:%d/management.html#/", port)
	return browser.OpenURL(url)
}

func loadTrayIcon() []byte {
	for _, candidate := range trayIconCandidates() {
		data, err := os.ReadFile(candidate)
		if err != nil || len(data) == 0 {
			continue
		}
		return data
	}
	if len(embeddedTrayIcon) > 0 {
		return embeddedTrayIcon
	}
	return nil
}

func trayIconCandidates() []string {
	baseDirs := make([]string, 0, 2)
	if exePath, err := os.Executable(); err == nil {
		baseDirs = append(baseDirs, filepath.Dir(exePath))
	}
	if wd, err := os.Getwd(); err == nil {
		baseDirs = append(baseDirs, wd)
	}

	seen := make(map[string]struct{}, 8)
	candidates := make([]string, 0, 8)
	addCandidate := func(path string) {
		if path == "" {
			return
		}
		resolved, err := filepath.Abs(path)
		if err != nil {
			return
		}
		key := strings.ToLower(resolved)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		candidates = append(candidates, resolved)
	}

	for _, baseDir := range baseDirs {
		addCandidate(filepath.Join(baseDir, "image", "logo-tray.ico"))
		addCandidate(filepath.Join(baseDir, "image", "logo.ico"))
		addCandidate(filepath.Join(baseDir, "tray.ico"))
	}

	return candidates
}
