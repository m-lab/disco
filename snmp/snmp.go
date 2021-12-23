package snmp

import (
	"github.com/gosnmp/gosnmp"
)

// Client defines a new SNMP interface to abstract SNMP operations.
type Client interface {
	BulkWalkAll(rootOid string) (results []gosnmp.SnmpPDU, err error)
	Get(oids []string) (result *gosnmp.SnmpPacket, err error)
}

// SwitchClient implements the Client interface.
type SwitchClient struct {
	GoSNMP *gosnmp.GoSNMP
}

// BulkWalkAll performs an SNMP BulkWalk operation for an OID, returning an
// array of all values.
func (s *SwitchClient) BulkWalkAll(rootOid string) (results []gosnmp.SnmpPDU, err error) {
	return s.GoSNMP.BulkWalkAll(rootOid)
}

// Get does an SNMP Get operation on an array of OIDs.
func (s *SwitchClient) Get(oids []string) (results *gosnmp.SnmpPacket, err error) {
	return s.GoSNMP.Get(oids)
}

// New returns a new SNMP client.
func New(s *gosnmp.GoSNMP) *SwitchClient {
	return &SwitchClient{
		GoSNMP: s,
	}
}
