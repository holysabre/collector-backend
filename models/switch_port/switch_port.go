package switch_port

import "collector-agent/models/snmp"

type SwitchPort struct {
	ID                   uint64                    `json:"id"`
	PortIndex            uint64                    `json:"port_index"`
	Oids                 map[string]string         `json:"-"`
	Pdus                 []snmp.Pdu                `json:"pdus"`
	Connection           snmp.SNMPConnectionConfig `json:"-"`
	RevertedOriginalOids map[string]string         `json:"-"`
}
