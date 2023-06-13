package switch_port

import (
	model_snmp "collector-agent/models/snmp"
	model_sp "collector-agent/models/switch_port"
	"log"

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

type SPCollector struct {
	Connection *g.GoSNMP
	SwitchPort *model_sp.SwitchPort
}

func NewSPCollector(conn *g.GoSNMP, sp *model_sp.SwitchPort, oids map[string]string) *SPCollector {
	sp.SetOids(oids)
	return &SPCollector{
		Connection: conn,
		SwitchPort: sp,
	}
}

func (spc *SPCollector) GetByOids() {
	oids := []string{}
	for _, oid := range spc.SwitchPort.Oids {
		oids = append(oids, oid)
	}

	result, err2 := spc.Connection.Get(oids) // Get() accepts up to g.MAX_OIDS
	if err2 != nil {
		log.Printf("Get() err: %v", err2)
		return
	}

	pdus := []model_snmp.Pdu{}

	for _, variable := range result.Variables {
		oid, value := model_snmp.GetSNMPValue(variable, true)
		key := spc.SwitchPort.RevertedOriginalOids[oid]
		pdu := model_snmp.Pdu{Oid: oid, Value: value, Key: key}
		// fmt.Println(pdu)
		pdus = append(pdus, pdu)
	}
	spc.SwitchPort.Pdus = pdus
}
