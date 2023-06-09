package connection

import (
	"log"
	"sync"

	g "github.com/gosnmp/gosnmp"
)

type SNMPConnectionPool struct {
	pool  *sync.Pool
	mutex sync.Mutex
}

func NewSNMPConnectionPool(params *g.GoSNMP) *SNMPConnectionPool {
	return &SNMPConnectionPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return createConnection(params)
			},
		},
	}
}

func (p *SNMPConnectionPool) GetConnection() *g.GoSNMP {
	return p.pool.Get().(*g.GoSNMP)
}

func (p *SNMPConnectionPool) ReleaseConnection(conn *g.GoSNMP) {
	p.pool.Put(conn)
}

func createConnection(params *g.GoSNMP) *g.GoSNMP {
	err := params.Connect()
	if err != nil {
		log.Fatalf("无法建立 SNMP 连接：%v", err)
	}

	return params
}
