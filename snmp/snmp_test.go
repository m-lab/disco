package snmp

import (
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
)

func Test_New(t *testing.T) {
	goSNMP := &gosnmp.GoSNMP{
		Target:    "s1-abc0t.measurement-lab.org",
		Port:      uint16(161),
		Community: "snmp-community",
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(2) * time.Second,
		Retries:   1,
	}

	client := New(goSNMP)
	var i interface{} = client
	_, ok := i.(Client)
	if !ok {
		t.Error("Expected return value of New() to implement interface Client.")
	}
}
