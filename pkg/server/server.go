package server

import (
	"collector-agent/util"
	"fmt"

	goipmi "github.com/vmware/goipmi"
)

type ServerCollector struct{}

func NewServerCollector() *ServerCollector {
	return &ServerCollector{}
}

func (sc *ServerCollector) Collect() {

	rootDir := util.GetBinDir()
	filename := rootDir + "/" + "ipmitool"

	conn := goipmi.Connection{
		Path:      filename,
		Hostname:  "192.168.109.2",
		Username:  "root",
		Password:  "root",
		Interface: "lanplus",
	}

	client, err := goipmi.NewClient(&conn)
	util.LogIfErr(err)

	defer client.Close()

	err = client.Open()
	util.LogIfErr(err)

	id, err := client.DeviceID()
	util.LogIfErr(err)

	fmt.Println(id)

}
