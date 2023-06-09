package network_switch

import (
	"collector-agent/device"
	"collector-agent/device/network_switch/switch_port"
	"collector-agent/util"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"time"

	g "github.com/gosnmp/gosnmp"
)

type NetworkSwitch struct {
	ID         uint64                   `json:"id"`
	Connection device.SNMPConnection    `json:"connection"`
	Oids       map[string]string        `json:"oids"`
	Ports      []switch_port.SwitchPort `json:"ports"`
	PortOids   map[string]string        `json:"port_oids"`
	Pdus       []device.Pdu             `json:"pdus"`
	Time       time.Time                `json:"time"`
}

func Collect(body []byte) NetworkSwitch {
	// fmt.Println(string(body))
	var ns NetworkSwitch
	err := json.Unmarshal(body, &ns)
	if err != nil {
		fmt.Printf("NetworkSwitch 无法解析JSON数据: %v", err)
		return ns
	}

	conn := &g.GoSNMP{
		Target:    ns.Connection.Target,
		Port:      uint16(ns.Connection.Port),
		Community: ns.Connection.Community,
		Version:   device.GetSNMPVersion(ns.Connection.Version),
		Timeout:   time.Duration(2) * time.Second,
		Retries:   0,
	}
	err = conn.Connect()
	if err != nil {
		fmt.Printf("无法建立 SNMP 连接：%v", err)
		return ns
	}
	defer conn.Conn.Close()

	// 获取设备信息
	resp, err := conn.Get([]string{".1.3.6.1.2.1.1.1.0", ".1.3.6.1.2.1.1.6.0"})
	if err != nil {
		log.Printf("ns %d SNMP 请求错误：%v", ns.ID, err)
		return ns
	}

	if resp.Error != g.NoError {
		log.Printf("SNMP 响应错误：%v", resp.Error)
		return ns
	}

	for _, oid := range ns.Oids {
		if oid == "" {
			continue
		}
		ns.WalkAllByOid(oid, conn)
	}
	fmt.Printf("ns %d collect oids finished\n", ns.ID)

	for index, port := range ns.Ports {
		port.Connection = ns.Connection
		port.SetOids(ns.PortOids)
		port.GetByOids(conn)
		ns.Ports[index] = port
	}
	fmt.Printf("ns %d collect ports oids finished\n", ns.ID)

	ns.Time = time.Now()

	return ns
}

func (ns *NetworkSwitch) WalkAllByOid(oid string, conn *g.GoSNMP) {
	variables, err2 := conn.WalkAll(oid)
	if err2 != nil {
		util.FailOnError(err2, "Get() err: %v")
		return
	}

	for _, variable := range variables {
		oid, value := device.GetSNMPValue(variable, true)
		val := value.(*big.Int)
		if val.Uint64() > 0 && val.Uint64() < 65535 {
			key := util.GetKeyByOid(ns.Oids, oid)
			pdu := device.Pdu{Oid: oid, Value: value, Key: key}
			ns.Pdus = append(ns.Pdus, pdu)
		}
	}
	ns.Time = time.Now()
}
