package switch_port

import (
	"collector-agent/util"
	"strconv"
)

func (sp *SwitchPort) SetOids(original_oids map[string]string) {
	oids := map[string]string{}
	for key, oid := range original_oids {
		oids[key] = oid + "." + strconv.Itoa(int(sp.PortIndex))
	}
	sp.Oids = oids
	sp.RevertedOriginalOids = util.ReverseMap(original_oids)
}
