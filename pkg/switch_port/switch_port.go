package switch_port

import (
	model_snmp "collector-agent/models/snmp"
	model_sp "collector-agent/models/switch_port"
	"collector-agent/pkg/logger"

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

	result, err := spc.Connection.Get(oids) // Get() accepts up to g.MAX_OIDS
	if err != nil {
		logger.Printf("ns port #%d Get() err: %v", spc.SwitchPort.ID, err.Error())
		return
	}

	if result.Error != g.NoError {
		errOid := oids[result.ErrorIndex-1]
		logger.Printf("ns port %d, oid: %s, err: %s \n", spc.SwitchPort.ID, errOid, result.Error.String())
		return
	}

	pdus := []model_snmp.Pdu{}
	for _, variable := range result.Variables {
		oid, value := model_snmp.GetSNMPValue(variable, true)
		key := spc.SwitchPort.RevertedOriginalOids[oid]
		pdu := model_snmp.Pdu{Oid: oid, Value: value, Key: key}
		pdus = append(pdus, pdu)
	}
	spc.SwitchPort.Pdus = pdus
}
