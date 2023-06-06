package switch_port

import (
	"collector-agent/device"
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
	ID        uint64       `json:"id"`
	PortIndex uint64       `json:"port_index"`
	Oids      []string     `json:"-"`
	Pdus      []device.Pdu `json:"-"`
	// Time       time.Time             `json:"-"`
	Connection device.SNMPConnection `json:"-"`
}

func (sp *SwitchPort) SetOids(original_oids []string) {
	oids := []string{}
	for _, oid := range original_oids {
		oids = append(oids, oid+"."+strconv.Itoa(int(sp.PortIndex)))
	}
	sp.Oids = oids
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

	result, err2 := g.Default.Get(sp.Oids) // Get() accepts up to g.MAX_OIDS
	if err2 != nil {
		log.Fatalf("Get() err: %v", err2)
	}

	pdus := []device.Pdu{}

	for _, variable := range result.Variables {
		oid, value := device.GetSNMPValue(variable, true)
		pdus = append(pdus, device.Pdu{Oid: oid, Value: value})
	}
	sp.Pdus = pdus
}
