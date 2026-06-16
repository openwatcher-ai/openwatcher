package processutil

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

func KillListeningProcess(listen string) (bool, error) {
	_, port, err := net.SplitHostPort(strings.TrimSpace(listen))
	if err != nil || strings.TrimSpace(port) == "" {
		return false, fmt.Errorf("监听地址不合法")
	}

	pids, err := listeningPIDs(port)
	if err != nil {
		return false, err
	}
	if len(pids) == 0 {
		return false, nil
	}

	var failed []string
	killed := false
	for _, pid := range pids {
		process, findErr := os.FindProcess(pid)
		if findErr != nil {
			failed = append(failed, fmt.Sprintf("%d", pid))
			continue
		}
		if killErr := process.Kill(); killErr != nil {
			failed = append(failed, fmt.Sprintf("%d", pid))
			continue
		}
		killed = true
	}

	if len(failed) > 0 {
		return killed, fmt.Errorf("停止端口占用进程失败：%s", strings.Join(failed, ", "))
	}
	return killed, nil
}

func listeningPIDs(port string) ([]int, error) {
	switch runtime.GOOS {
	case "windows":
		return listeningPIDsFromNetstat(port)
	default:
		return listeningPIDsFromLsof(port)
	}
}

func listeningPIDsFromLsof(port string) ([]int, error) {
	output, err := exec.Command("lsof", "-nP", "-tiTCP:"+strings.TrimSpace(port), "-sTCP:LISTEN").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	return parsePIDList(string(output)), nil
}

func listeningPIDsFromNetstat(port string) ([]int, error) {
	output, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return nil, err
	}

	var pids []int
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(fields) < 5 {
			continue
		}
		localAddress := fields[1]
		state := strings.ToUpper(fields[3])
		pidField := fields[4]
		if state != "LISTENING" {
			continue
		}
		if !strings.HasSuffix(localAddress, ":"+strings.TrimSpace(port)) {
			continue
		}
		pid, convErr := strconv.Atoi(strings.TrimSpace(pidField))
		if convErr != nil || pid <= 0 {
			continue
		}
		pids = append(pids, pid)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	slices.Sort(pids)
	return slices.Compact(pids), nil
}

func parsePIDList(raw string) []int {
	var pids []int
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		pid, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil || pid <= 0 {
			continue
		}
		pids = append(pids, pid)
	}
	slices.Sort(pids)
	return slices.Compact(pids)
}
