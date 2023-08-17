package system

import (
	"bytes"
	model_system "collector-agent/models/system"
	"collector-agent/pkg/logger"
	"collector-agent/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

const CmdTimeout = 5 * time.Second

type SystemCollector struct {
	SystemInfo *model_system.SystemInfo
}

func NewSystemCollector(s *model_system.SystemInfo) *SystemCollector {
	logger.Printf("sys %d starting", s.ID)
	return &SystemCollector{
		SystemInfo: s,
	}
}

func (sc *SystemCollector) Collect() error {
	time.Sleep(10 * time.Second)

	err := sc.collectIOStat()
	logger.LogIfErr(err)

	err = sc.collectRam()
	logger.LogIfErr(err)

	err = sc.collectDisk()
	logger.LogIfErr(err)

	err = sc.collectNet()
	logger.LogIfErr(err)

	sc.SystemInfo.Time = time.Now()

	// fmt.Println("collect done \n", sc.SystemInfo)
	return nil
}

func (sc *SystemCollector) collectIOStat() error {
	args := []string{"-x", "-o", "JSON", "1", "2"}
	out, err := sc.run("iostat", args)
	if err != nil {
		return err
	}

	var iostat model_system.IoStat
	if err := json.Unmarshal([]byte(out), &iostat); err != nil {
		logger.Println("Failed to unmarshal JSON")
		return err
	}

	if len(iostat.Sysstat.Hosts) > 0 {
		firstHost := iostat.Sysstat.Hosts[0]
		if len(firstHost.Statistics) > 0 {
			currentStatistic := firstHost.Statistics[0]
			percentage := 100 - currentStatistic.AvgCpu.Idel
			percentage = float32(util.RoundFloat(float64(percentage), 0))
			cpuParame := model_system.Parame{
				Key:   "cpu",
				Value: map[string]interface{}{"percentage": percentage},
			}
			sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, cpuParame)

			disks := map[string]map[string]interface{}{}
			for _, d := range currentStatistic.Disks {
				disk := map[string]interface{}{}
				disk["kB_wrtn/s"] = d.WriteKBS
				disk["kB_read/s"] = d.ReadKBS
				disk["util"] = d.Util
				disks[d.DiskDevice] = disk
			}
			disksParame := model_system.Parame{
				Key:   "io",
				Value: disks,
			}
			sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, disksParame)
			// fmt.Println(sc.SystemInfo.Parames)
		}
	}
	return nil
}

func (sc *SystemCollector) collectRam() error {
	vm, err := mem.VirtualMemory()
	if err != nil {
		errStr := fmt.Sprintf("Failed to get virtual memory info, err: %v \n", err.Error())
		return errors.New(errStr)
	}

	disksParame := model_system.Parame{
		Key: "ram",
		Value: map[string]interface{}{
			"total":      float32(vm.Total / 1024 / 1024),
			"used":       float32(vm.Used / 1024 / 1024),
			"percentage": float32(util.RoundFloat(vm.UsedPercent, 0)),
		},
	}
	sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, disksParame)
	return nil
}

func (sc *SystemCollector) collectDisk() error {
	args := []string{"-c", `mount | grep /app | grep -v iso | grep -v /app/run | awk '{print $3}'`}
	out, err := sc.run("bash", args)
	if err != nil {
		errStr := fmt.Sprintf("Failed to get disk info, err: %v \n", err.Error())
		return errors.New(errStr)
	}

	lines := bytes.Split(out, []byte{'\n'})
	diskPath := map[string]map[string]interface{}{}
	var total uint64
	var used uint64
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		path := string(line)
		usage, err := disk.Usage(path)
		if err != nil {
			logger.Printf("Error: %v\n", err.Error())
			continue
		}

		usageStat := map[string]interface{}{
			"total":      usage.Total,
			"used":       usage.Used,
			"percentage": usage.UsedPercent,
		}

		diskPath[path] = usageStat

		total += usage.Total
		used += usage.Used
	}

	var percentage float32
	if used > 0 && total > 0 {
		percentage = float32(used / total * 100)
	}

	disksParame := model_system.Parame{
		Key: "disk",
		Value: map[string]interface{}{
			"total":      float32(total),
			"used":       float32(used),
			"percentage": percentage,
		},
	}
	sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, disksParame)

	diskPathParame := model_system.Parame{
		Key:   "disk_path",
		Value: diskPath,
	}
	sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, diskPathParame)
	return nil
}

func (sc *SystemCollector) collectNet() error {
	ioCountersStat, err := net.IOCounters(true)
	if err != nil {
		errStr := fmt.Sprintf("Fail To Get Network Data %v: \n", err.Error())
		return errors.New(errStr)
	}

	network := map[string]map[string]interface{}{}
	for _, ioCounterStat := range ioCountersStat {
		if ioCounterStat.Name == "veth" || ioCounterStat.Name == "br-" || ioCounterStat.Name == "docker" {
			continue
		}
		// fmt.Printf("%s out: %d, in: %d \n", ioCounterStat.Name, ioCounterStat.BytesSent, ioCounterStat.BytesRecv)
		ioStat := map[string]interface{}{
			"in":  float32(ioCounterStat.BytesRecv),
			"out": float32(ioCounterStat.BytesSent),
		}

		network[ioCounterStat.Name] = ioStat
	}

	diskPathParame := model_system.Parame{
		Key:   "network",
		Value: network,
	}
	sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, diskPathParame)
	return nil
}

func (sc *SystemCollector) run(command string, args []string) ([]byte, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, CmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	// fmt.Println("cmd: ", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}
