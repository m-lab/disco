package snmp

import (
	"github.com/soniah/gosnmp"
)

// SNMP defines a new SNMP interface to abstract SNMP operations.
type SNMP interface {
	BulkWalkAll(rootOid string) (results []gosnmp.SnmpPDU, err error)
	Get(oids []string) (result *gosnmp.SnmpPacket, err error)
}

// RealSNMP implements the SNMP interface.
type RealSNMP struct {
	GoSNMP *gosnmp.GoSNMP
}

// BulkWalkAll performs an SNMP BulkWalk operation for an OID, returning an
// array of all values.
func (s *RealSNMP) BulkWalkAll(rootOid string) (results []gosnmp.SnmpPDU, err error) {
	return s.GoSNMP.BulkWalkAll(rootOid)
}

// Get does an SNMP Get operation on an array of OIDs.
func (s *RealSNMP) Get(oids []string) (results *gosnmp.SnmpPacket, err error) {
	return s.GoSNMP.Get(oids)
}

// Client returns a new RealSNMP object.
func Client(s *gosnmp.GoSNMP) *RealSNMP {
	return &RealSNMP{
		GoSNMP: s,
	}
}
