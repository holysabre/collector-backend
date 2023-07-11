package network_switch

import (
	model_ns "collector-agent/models/network_switch"
	model_snmp "collector-agent/models/snmp"
	"collector-agent/pkg/switch_port"
	"collector-agent/util"
	"fmt"
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
	conn := &g.GoSNMP{
		Target:    ns.Connection.Target,
		Port:      uint16(ns.Connection.Port),
		Community: ns.Connection.Community,
		Version:   model_snmp.GetSNMPVersion(ns.Connection.Version),
		Timeout:   time.Duration(2) * time.Second,
		Retries:   0,
	}
	return &NSCollector{
		Connection:    conn,
		NetworkSwitch: ns,
	}
}

func (nsc *NSCollector) Collect() error {
	err := nsc.Connection.Connect()
	if err != nil {
		fmt.Printf("无法建立 SNMP 连接：%v", err)
		return err
	}
	defer nsc.Connection.Conn.Close()

	// 获取设备信息
	resp, err := nsc.Connection.Get([]string{".1.3.6.1.2.1.1.1.0", ".1.3.6.1.2.1.1.6.0"})
	if err != nil {
		log.Printf("ns %d SNMP 请求错误：%v", nsc.NetworkSwitch.ID, err)
		return err
	}

	if resp.Error != g.NoError {
		log.Printf("SNMP 响应错误：%v", resp.Error)
		return err
	}

	for _, oid := range nsc.NetworkSwitch.Oids {
		if oid == "" {
			continue
		}
		nsc.WalkAllByOid(oid)
	}
	fmt.Printf("ns %d collect oids finished\n", nsc.NetworkSwitch.ID)

	for index, sp := range nsc.NetworkSwitch.Ports {
		spc := switch_port.NewSPCollector(nsc.Connection, &sp, nsc.NetworkSwitch.PortOids)
		spc.Connection = nsc.Connection
		spc.GetByOids()
		nsc.NetworkSwitch.Ports[index] = sp
	}
	fmt.Printf("ns %d collect ports oids finished\n", nsc.NetworkSwitch.ID)

	nsc.NetworkSwitch.Time = time.Now()

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
