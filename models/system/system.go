package system

import "time"

type SystemInfo struct {
	ID      uint64    `json:"id"`
	Parames []Parame  `json:"parames"`
	Time    time.Time `json:"time"`
}

type Parame struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type Cpu struct {
	Percentage float32 `json:"percentage"`
}

type Ram struct {
	Total      float32 `json:"total"`
	Used       float32 `json:"used"`
	Percentage float32 `json:"percentage"`
}

type DiskCap struct {
	Name       string  `json:"name"`
	Total      float32 `json:"total"`
	Used       float32 `json:"used"`
	Percentage float32 `json:"percentage"`
}

type IoStat struct {
	Sysstat Sysstat `json:"sysstat"`
}

type NetIO struct {
	Name string  `json:"name"`
	In   float32 `json:"in"`
	Out  float32 `json:"out"`
}

type Sysstat struct {
	Hosts []Host `json:"hosts"`
}

type Host struct {
	Nodename     string      `json:"nodename"`
	Sysname      string      `json:"sysname"`
	Release      string      `json:"release"`
	Machine      string      `json:"machine"`
	NumberOfCpus int         `json:"number-of-cpus"`
	Date         string      `json:"date"`
	Statistics   []Statistic `json:"statistics"`
}

type Statistic struct {
	AvgCpu AvgCpu `json:"avg-cpu"`
	Disks  []Disk `json:"disk"`
}

type AvgCpu struct {
	User   float32 `json:"user"`
	Nice   float32 `json:"nice"`
	System float32 `json:"system"`
	Iowait float32 `json:"iowait"`
	Steal  float32 `json:"steal"`
	Idel   float32 `json:"idle"`
}

type Disk struct {
	DiskDevice string  `json:"disk_device"`
	WriteKBS   float32 `json:"wkB/s"`
	ReadKBS    float32 `json:"rkB/s"`
	Util       float32 `json:"util"`
}
