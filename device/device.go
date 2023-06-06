package device

import (
	"strings"

	g "github.com/gosnmp/gosnmp"
)

type SNMPConnection struct {
	Target    string `json:"target"`
	Port      int    `json:"port"`
	Community string `json:"community"`
	Version   string `json:"version"`
}

type Pdu struct {
	Oid   string      `json:"oid"`
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func GetSNMPVersion(v string) (version g.SnmpVersion) {
	switch v {
	case "1":
		version = g.Version1
	case "3":
		version = g.Version3
	case "2c":
	default:
		version = g.Version2c
	}
	return
}

func GetSNMPValue(variable g.SnmpPDU, is_port bool) (oid string, value interface{}) {
	name, _ := strings.CutPrefix(variable.Name, ".")
	if is_port {
		index := strings.LastIndex(name, ".")
		if index == -1 {
			return
		}
		oid = name[:index]
	} else {
		oid = name
	}

	switch variable.Type {
	case g.OctetString:
		bytes := variable.Value.([]byte)
		value = string(bytes)
	default:
		value = g.ToBigInt(variable.Value)
	}
	return
}

func GetKeyFromOid(oid string, reverted_oids map[string]string) string {
	return reverted_oids[oid]
}
