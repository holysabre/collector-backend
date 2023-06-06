package switch_port

import (
	"collector-agent/device"
	"collector-agent/util"
	"fmt"
	"log"
	"strconv"

	g "github.com/gosnmp/gosnmp"
)

type Status uint8

const (
	StatusUp           Status = 1
	StatusDown         Status = 2
	StatusTesting      Status = 3
	StatusUnknown      Status = 4
	StatusDormant      Status = 5
	StatusNotPresent   Status = 6
	StatusLowLayerDown Status = 7
)

type SwitchPort struct {
	ID                   uint64                `json:"id"`
	PortIndex            uint64                `json:"port_index"`
	Oids                 map[string]string     `json:""`
	Pdus                 []device.Pdu          `json:"pdus"`
	Connection           device.SNMPConnection `json:"-"`
	RevertedOriginalOids map[string]string     `json:"-"`
}

func (sp *SwitchPort) SetOids(original_oids map[string]string) {
	oids := map[string]string{}
	for key, oid := range original_oids {
		oids[key] = oid + "." + strconv.Itoa(int(sp.PortIndex))
	}
	sp.Oids = oids
	sp.RevertedOriginalOids = util.ReverseMap(original_oids)
}

func (sp *SwitchPort) GetByOids() {
	// Default is a pointer to a GoSNMP struct that contains sensible defaults
	// eg port 161, community public, etc
	g.Default.Target = sp.Connection.Target
	g.Default.Port = uint16(sp.Connection.Port)
	g.Default.Community = sp.Connection.Community
	g.Default.Version = device.GetSNMPVersion(sp.Connection.Version)
	err := g.Default.Connect()
	if err != nil {
		log.Fatalf("Connect() err: %v", err)
	}
	defer g.Default.Conn.Close()

	oids := []string{}
	for _, oid := range sp.Oids {
		oids = append(oids, oid)
	}

	result, err2 := g.Default.Get(oids) // Get() accepts up to g.MAX_OIDS
	if err2 != nil {
		log.Fatalf("Get() err: %v", err2)
	}

	pdus := []device.Pdu{}

	for _, variable := range result.Variables {
		oid, value := device.GetSNMPValue(variable, true)
		key := sp.RevertedOriginalOids[oid]
		pdu := device.Pdu{Oid: oid, Value: value, Key: key}
		fmt.Println(pdu)
		pdus = append(pdus, pdu)
	}
	sp.Pdus = pdus
}
