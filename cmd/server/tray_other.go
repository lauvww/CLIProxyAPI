//go:build !windows

package main

import (
	"fmt"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func shouldAutoTrayLaunch(_ []string) bool {
	return false
}

func shouldHideTrayConsole(_ bool, _ bool) bool {
	return false
}

func hideConsoleWindow() {}

func detachConsoleWindow() {}

func runTrayMode(_ *config.Config, _, _ string, _ bool) error {
	return fmt.Errorf("tray mode is only supported on Windows")
}
