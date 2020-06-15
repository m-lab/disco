package snmp

import (
	"testing"
	"time"

	"github.com/soniah/gosnmp"
)

func Test_Client(t *testing.T) {
	goSNMP := &gosnmp.GoSNMP{
		Target:    "s1-abc0t.measurement-lab.org",
		Port:      uint16(161),
		Community: "snmp-community",
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(2) * time.Second,
		Retries:   1,
	}

	client := Client(goSNMP)
	var i interface{} = client
	_, ok := i.(SNMP)
	if !ok {
		t.Error("Expected return value of Client() to implement interface SNMP.")
	}
}
