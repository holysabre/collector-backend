package server

import (
	"collector-agent/util"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	model_ipmi "collector-agent/models/ipmi"
	model_server "collector-agent/models/server"
)

const CmdTimeout = 5 * time.Second

type ServerCollector struct {
	Connection model_ipmi.Connection
	Server     *model_server.Server
}

func NewServerCollector(s *model_server.Server) *ServerCollector {
	return &ServerCollector{
		Server:     s,
		Connection: s.Connection,
	}
}

func (sc *ServerCollector) Collect() {
	// fmt.Printf("collect #%d start \n", sc.Server.ID)
	status, err := sc.getPower()
	if err != nil {
		fmt.Printf("server #%d get power failed, err: %v \n", sc.Server.ID, err.Error())
	} else {
		sc.Server.PowerStatus = status
	}

	power, err := sc.PowerReading()
	if err != nil {
		fmt.Printf("server #%d get power reading failed, err: %v \n", sc.Server.ID, err.Error())
	} else {
		sc.Server.PowerReading = power
	}

	// fmt.Println("collect done")
}

func (sc *ServerCollector) getPower() (string, error) {
	status := "Unknown"
	args := []string{"power", "status"}
	out, err := sc.run("ipmitool", args)
	if err != nil {
		return status, err
	}
	// fmt.Println(string(out))

	pattern := `Chassis Power is (on|off)`
	reg := regexp.MustCompile(pattern)
	matches := reg.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return status, err
	}
	status = matches[1]
	// fmt.Printf("server #%d power status: %s \n", sc.Server.ID, status)

	sc.Server.Time = time.Now()

	return status, nil
}

func (sc *ServerCollector) PowerReading() (int, error) {
	args := []string{"dcmi", "power", "reading"}

	var out []byte
	var err error
	out, err = sc.run("ipmitool", args)
	if err != nil {
		return 0, err
	}

	pattern := `Instantaneous power reading:\s+([\d.]+)\s+Watts`
	reg := regexp.MustCompile(pattern)
	matches := reg.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		power, _ := strconv.Atoi(matches[1])
		// fmt.Printf("server #%d power reading: %d \n", sc.Server.ID, power)
		return power, nil
	}

	if strings.ContainsAny(string(out), "DCMI request failed") {
		fmt.Println("run old ipmitool")
		oldArgs := []string{"sdr", "get", "Power"}
		rootDir := util.GetBinDir()
		oldCommand := rootDir + "/ipmitool"
		out, err = sc.run(oldCommand, oldArgs)
		if err != nil {
			return 0, err
		}
		pattern = `^\s*Sensor Reading\s+:\s+(\d+)`
		reg = regexp.MustCompile(pattern)
		matches = reg.FindStringSubmatch(string(out))
		if len(matches) > 1 {
			power, _ := strconv.Atoi(matches[1])
			return power, nil
		}
	}

	return 0, errors.New("cannot get power")
}

func (sc *ServerCollector) run(command string, appendArgs []string) ([]byte, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, CmdTimeout)
	defer cancel()

	args := []string{"-R", "2", "-H", sc.Connection.Hostname, "-U", sc.Connection.Username, "-P", sc.Connection.Password, "-I", "lanplus"}
	args = append(args, appendArgs...)
	cmd := exec.CommandContext(ctx, command, args...)
	// fmt.Println("cmd: ", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}
