package network_switch

import (
	"collector-agent/models/snmp"
	"collector-agent/models/switch_port"
	"time"
)

type NetworkSwitch struct {
	ID         uint64                    `json:"id"`
	Connection snmp.SNMPConnectionConfig `json:"connection"`
	Oids       map[string]string         `json:"oids"`
	Ports      []switch_port.SwitchPort  `json:"ports"`
	PortOids   map[string]string         `json:"port_oids"`
	Pdus       []snmp.Pdu                `json:"pdus"`
	Time       time.Time                 `json:"time"`
}
