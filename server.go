package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func appDataDir(dirName string) string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	dir := filepath.Join(appData, dirName, "data")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func serverBinaryPath(isEnhanced bool) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)

	candidateName := "DrizzleGatewayServer.exe"
	if isEnhanced {
		candidateName = "DrizzleGatewayServer-Enhanced.exe"
	}

	candidate := filepath.Join(dir, candidateName)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	standardCandidate := filepath.Join(dir, "DrizzleGatewayServer.exe")
	if _, err := os.Stat(standardCandidate); err == nil {
		return standardCandidate, nil
	}
	if _, err := os.Stat(candidateName); err == nil {
		abs, _ := filepath.Abs(candidateName)
		return abs, nil
	}
	return "", fmt.Errorf("DrizzleGatewayServer.exe not found alongside executable")
}

func isHealthy(port int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url)
	if err == nil {
		resp.Body.Close()
		return resp.StatusCode == 200
	}
	return false
}

func waitHealthy(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isHealthy(port) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func startServer(serverBin string, port int, storeDir string) (*exec.Cmd, error) {
	cmd := exec.Command(serverBin)
	cmd.Dir = filepath.Dir(serverBin)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"STORE_PATH="+storeDir,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func stopServers(serverBinName string) {
	killCmd := exec.Command("taskkill", "/F", "/T", "/IM", serverBinName)
	killCmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_ = killCmd.Run()
}
