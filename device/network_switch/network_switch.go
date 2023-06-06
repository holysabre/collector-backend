package network_switch

import (
	"collector-agent/device"
	"collector-agent/device/network_switch/switch_port"
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
	Pdus       []device.Pdu             `json:"-"`
	Time       time.Time                `json:"-"`
}

func Collect(body []byte) (ns NetworkSwitch) {
	// fmt.Println(string(body))
	err := json.Unmarshal(body, &ns)
	if err != nil {
		fmt.Printf("NetworkSwitch 无法解析JSON数据: %v", err)
		return
	}

	for _, oid := range ns.Oids {
		if oid == "" {
			continue
		}
		ns.WalkAllByOid(oid)
	}

	// port
	for _, port := range ns.Ports {
		port.Connection = ns.Connection
		port.SetOids(ns.PortOids)
		port.GetByOids()
	}

	fmt.Println(ns)

	return
}

func (ns *NetworkSwitch) WalkAllByOid(oid string) {
	g.Default.Target = ns.Connection.Target
	g.Default.Port = uint16(ns.Connection.Port)
	g.Default.Community = ns.Connection.Community
	g.Default.Version = device.GetSNMPVersion(ns.Connection.Version)
	err := g.Default.Connect()
	if err != nil {
		log.Fatalf("Connect() err: %v", err)
	}
	defer g.Default.Conn.Close()

	variables, err2 := g.Default.WalkAll(oid)
	if err2 != nil {
		log.Fatalf("Get() err: %v", err2)
	}

	for _, variable := range variables {
		oid, value := device.GetSNMPValue(variable, true)
		val := value.(*big.Int)
		if val.Uint64() > 0 && val.Uint64() < 65535 {
			pdu := device.Pdu{Oid: oid, Value: value}
			ns.Pdus = append(ns.Pdus, pdu)
		}
	}
	ns.Time = time.Now()
}
