package network_switch

import (
	model_ns "collector-agent/models/network_switch"
	model_snmp "collector-agent/models/snmp"
	"collector-agent/pkg/logger"
	"collector-agent/pkg/switch_port"
	"collector-agent/util"
	"log"
	"math/big"
	"time"

	g "github.com/gosnmp/gosnmp"
)

type NSCollector struct {
	Connection    *g.GoSNMP
	NetworkSwitch *model_ns.NetworkSwitch
}

func NewNSCollector(ns *model_ns.NetworkSwitch) *NSCollector {
	logger.Printf("ns %d starting", ns.ID)
	conn := &g.GoSNMP{
		Target:    ns.Connection.Target,
		Port:      uint16(ns.Connection.Port),
		Community: ns.Connection.Community,
		Version:   model_snmp.GetSNMPVersion(ns.Connection.Version),
		Timeout:   time.Duration(10) * time.Second,
		Retries:   2,
		MaxOids:   1000,
	}
	return &NSCollector{
		Connection:    conn,
		NetworkSwitch: ns,
	}
}

func (nsc *NSCollector) Collect() error {
	err := nsc.Connection.Connect()
	if err != nil {
		logger.Printf("ns #%d Unable To Connect ns Via SNMP: %v \n", nsc.NetworkSwitch.ID, err.Error())
		return err
	}
	defer nsc.Connection.Conn.Close()

	// 获取设备信息
	resp, err := nsc.Connection.Get([]string{".1.3.6.1.2.1.1.1.0", ".1.3.6.1.2.1.1.6.0"})
	if err != nil {
		log.Println(nsc.NetworkSwitch.Connection)
		log.Println(nsc.Connection)
		logger.Printf("ns #%d SNMP request err: %v \n", nsc.NetworkSwitch.ID, err.Error())
		return err
	}

	if resp.Error != g.NoError {
		logger.Printf("ns #%d SNMP response err: %v\n", nsc.NetworkSwitch.ID, resp.Error)
		return err
	}

	for _, oid := range nsc.NetworkSwitch.Oids {
		if oid == "" {
			continue
		}
		nsc.WalkAllByOid(oid)
	}
	logger.Printf("ns %d collect oids finished\n", nsc.NetworkSwitch.ID)

	for index, sp := range nsc.NetworkSwitch.Ports {
		spc := switch_port.NewSPCollector(nsc.Connection, &sp, nsc.NetworkSwitch.PortOids)
		spc.Connection = nsc.Connection
		spc.GetByOids()
		nsc.NetworkSwitch.Ports[index] = sp
	}
	logger.Printf("ns %d collect ports oids finished\n", nsc.NetworkSwitch.ID)

	nsc.NetworkSwitch.Time = time.Now()
	logger.Printf("ns %d finished", nsc.NetworkSwitch.ID)
	return nil
}

func (nsc *NSCollector) WalkAllByOid(oid string) error {
	variables, err2 := nsc.Connection.WalkAll(oid)
	if err2 != nil {
		util.FailOnError(err2, "Get() err")
		return err2
	}

	for _, variable := range variables {
		oid, value := model_snmp.GetSNMPValue(variable, true)
		val := value.(*big.Int)
		if val.Uint64() > 0 && val.Uint64() < 65535 {
			key := util.GetKeyByOid(nsc.NetworkSwitch.Oids, oid)
			pdu := model_snmp.Pdu{Oid: oid, Value: value, Key: key}
			nsc.NetworkSwitch.Pdus = append(nsc.NetworkSwitch.Pdus, pdu)
		}
	}
	nsc.NetworkSwitch.Time = time.Now()

	return nil
}
