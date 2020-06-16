package snmp

import (
	"github.com/soniah/gosnmp"
)

// SNMP defines a new SNMP interface to abstract SNMP operations.
type SNMP interface {
	BulkWalkAll(rootOid string) (results []gosnmp.SnmpPDU, err error)
	Get(oids []string) (result *gosnmp.SnmpPacket, err error)
}

// Client implements the SNMP interface.
type Client struct {
	GoSNMP *gosnmp.GoSNMP
}

// BulkWalkAll performs an SNMP BulkWalk operation for an OID, returning an
// array of all values.
func (s *Client) BulkWalkAll(rootOid string) (results []gosnmp.SnmpPDU, err error) {
	return s.GoSNMP.BulkWalkAll(rootOid)
}

// Get does an SNMP Get operation on an array of OIDs.
func (s *Client) Get(oids []string) (results *gosnmp.SnmpPacket, err error) {
	return s.GoSNMP.Get(oids)
}

// New returns a new SNMP client.
func New(s *gosnmp.GoSNMP) *Client {
	return &Client{
		GoSNMP: s,
	}
}
