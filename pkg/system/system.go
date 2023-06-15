package system

import (
	"bytes"
	model_system "collector-agent/models/system"
	"collector-agent/util"
	"context"
	"encoding/json"
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
	return &SystemCollector{
		SystemInfo: s,
	}
}

func (sc *SystemCollector) Collect() {
	sc.collectIOStat()
	sc.collectRam()
	sc.collectDisk()
	sc.collectNet()
	sc.SystemInfo.Time = time.Now()

	fmt.Println("collect done \n", sc.SystemInfo)
}

func (sc *SystemCollector) collectIOStat() {
	args := []string{"-x", "-o", "JSON", "1", "2"}
	out, err := sc.run("iostat", args)
	util.LogIfErr(err)

	var iostat model_system.IoStat
	if err := json.Unmarshal([]byte(out), &iostat); err != nil {
		fmt.Println("Failed to unmarshal JSON:", err)
	}

	if len(iostat.Sysstat.Hosts) > 0 {
		firstHost := iostat.Sysstat.Hosts[0]
		if len(firstHost.Statistics) > 0 {
			currentStatistic := firstHost.Statistics[0]

			cpuParame := model_system.Parame{
				Key:   "cpu",
				Value: map[string]interface{}{"percentage": 100 - currentStatistic.AvgCpu.Idel},
			}
			sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, cpuParame)

			disks := map[string]map[string]interface{}{}
			for _, d := range currentStatistic.Disks {
				disk := map[string]interface{}{}
				disk["wkB/s"] = d.WriteKBS
				disk["rkB/s"] = d.ReadKBS
				disk["util"] = d.Util
				disks[d.DiskDevice] = disk
			}
			disksParame := model_system.Parame{
				Key:   "disks",
				Value: disks,
			}
			sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, disksParame)
			fmt.Println(sc.SystemInfo.Parames)
		}
	}
}

func (sc *SystemCollector) collectRam() {
	vm, err := mem.VirtualMemory()
	if err != nil {
		fmt.Println("Failed to get virtual memory info:", err)
		panic(err)
	}

	disksParame := model_system.Parame{
		Key: "disks",
		Value: map[string]interface{}{
			"total":      float32(vm.Total / 1024 / 1024),
			"used":       float32(vm.Used / 1024 / 1024),
			"percentage": float32(vm.UsedPercent),
		},
	}
	sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, disksParame)
}

func (sc *SystemCollector) collectDisk() {
	args := []string{"-c", `mount | grep /app | grep -v iso | grep -v /app/run | awk '{print $3}'`}
	out, err := sc.run("bash", args)
	util.LogIfErr(err)

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
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Println(usage)

		usageStat := map[string]interface{}{
			"total":      usage.Total,
			"used":       usage.Used,
			"percentage": usage.UsedPercent,
		}

		diskPath[path] = usageStat

		total += usage.Total
		used += usage.Used
	}
	percentage := used / total * 100

	disksParame := model_system.Parame{
		Key: "disk",
		Value: map[string]interface{}{
			"total":      float32(total / 1024 / 1024),
			"used":       float32(used / 1024 / 1024),
			"percentage": float32(percentage),
		},
	}
	sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, disksParame)

	diskPathParame := model_system.Parame{
		Key:   "disk_path",
		Value: diskPath,
	}
	sc.SystemInfo.Parames = append(sc.SystemInfo.Parames, diskPathParame)
}

func (sc *SystemCollector) collectNet() {
	ioCountersStat, err := net.IOCounters(true)
	if err != nil {
		fmt.Println("获取网络接口信息失败:", err)
		return
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
}

func (sc *SystemCollector) run(command string, args []string) ([]byte, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, CmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	fmt.Println("cmd: ", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}
