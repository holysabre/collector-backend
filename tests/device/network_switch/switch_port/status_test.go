package switch_port_test

import (
	"collector-agent/device/network_switch"
	"encoding/json"
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	body := `{"type":"switch","id":1,"connection":{"target":"10.10.10.252","port":161,"community":"jvniu@123","version":"2c"},"oids":["1.3.6.1.4.1.25506.2.6.1.1.1.1.6","1.3.6.1.4.1.25506.2.6.1.1.1.1.8","1.3.6.1.4.1.25506.8.35.9.1.3.1.3"],"port_oids":["1.3.6.1.2.1.31.1.1.1.18","1.3.6.1.2.1.2.2.1.7","1.3.6.1.2.1.2.2.1.8","1.3.6.1.2.1.31.1.1.1.6","1.3.6.1.2.1.31.1.1.1.10"],"ports":[{"id":1,"port_index":1},{"id":2,"port_index":2},{"id":3,"port_index":3},{"id":4,"port_index":4},{"id":5,"port_index":5},{"id":6,"port_index":6},{"id":7,"port_index":7},{"id":8,"port_index":8},{"id":9,"port_index":9},{"id":10,"port_index":10},{"id":11,"port_index":11},{"id":12,"port_index":12},{"id":13,"port_index":13},{"id":14,"port_index":14},{"id":15,"port_index":15},{"id":16,"port_index":16},{"id":17,"port_index":17},{"id":18,"port_index":18},{"id":19,"port_index":19},{"id":20,"port_index":20},{"id":21,"port_index":21},{"id":22,"port_index":22},{"id":23,"port_index":23},{"id":24,"port_index":24},{"id":25,"port_index":25},{"id":26,"port_index":26},{"id":27,"port_index":27},{"id":28,"port_index":28},{"id":29,"port_index":29},{"id":30,"port_index":30},{"id":31,"port_index":31},{"id":32,"port_index":32},{"id":33,"port_index":33},{"id":34,"port_index":34},{"id":35,"port_index":35},{"id":36,"port_index":36},{"id":37,"port_index":37},{"id":38,"port_index":38},{"id":39,"port_index":39},{"id":40,"port_index":40},{"id":41,"port_index":41},{"id":42,"port_index":42},{"id":43,"port_index":43},{"id":44,"port_index":44},{"id":45,"port_index":45},{"id":46,"port_index":46},{"id":47,"port_index":47},{"id":48,"port_index":48},{"id":49,"port_index":50},{"id":50,"port_index":51},{"id":51,"port_index":52},{"id":52,"port_index":53},{"id":53,"port_index":57}],"time":1686015383}`
	var network_switch network_switch.NetworkSwitch
	err := json.Unmarshal([]byte(body), &network_switch)
	if err != nil {
		fmt.Printf("NetworkSwitch 无法解析JSON数据: %v", err)
		return
	}

	for _, port := range network_switch.Ports {
		port.Connection = network_switch.Connection
		port.SetOids(network_switch.PortOids)
		port.GetByOids()
		fmt.Println(port)
	}

	for _, oid := range network_switch.Oids {
		network_switch.WalkAllByOid(oid)
	}
}
