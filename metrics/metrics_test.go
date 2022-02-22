package metrics

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/m-lab/disco/archive"
	"github.com/m-lab/disco/config"
	"github.com/m-lab/go/rtx"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	sysUpTimeOID            = ".1.3.6.1.2.1.1.3.0"
	ifDescrMachineOID       = ".1.3.6.1.2.1.2.2.1.2.524"
	ifDescrUplinkOID        = ".1.3.6.1.2.1.2.2.1.2.568"
	ifHCInOctetsOidStub     = ".1.3.6.1.2.1.31.1.1.1.6"
	ifHCInOctetsMachineOID  = ".1.3.6.1.2.1.31.1.1.1.6.524"
	ifHCInOctetsUplinkOID   = ".1.3.6.1.2.1.31.1.1.1.6.568"
	ifOutDiscardsOidStub    = ".1.3.6.1.2.1.2.2.1.19"
	ifOutDiscardsMachineOID = ".1.3.6.1.2.1.2.2.1.19.524"
	ifOutDiscardsUplinkOID  = ".1.3.6.1.2.1.2.2.1.19.568"
)

var target = "s1-abc0t.measurement-lab.org"
var hostname = "mlab2-abc0t.mlab-sandbox.measurement-lab.org"
var machine = "mlab2"

var c = config.Config{
	Metrics: []config.Metric{
		config.Metric{
			Name:            "ifHCInOctets",
			Description:     "Ingress octets.",
			OidStub:         ifHCInOctetsOidStub,
			MlabUplinkName:  "switch.octets.uplink.rx",
			MlabMachineName: "switch.octets.local.rx",
		},
		config.Metric{
			Name:            "ifOutDiscards",
			Description:     "Egress discards.",
			OidStub:         ifOutDiscardsOidStub,
			MlabUplinkName:  "switch.discards.uplink.tx",
			MlabMachineName: "switch.discards.local.tx",
		},
	},
}

var snmpPacketMachine = gosnmp.SnmpPacket{
	Variables: []gosnmp.SnmpPDU{
		{
			Name:  ifDescrMachineOID,
			Type:  gosnmp.OctetString,
			Value: []byte("xe-0/0/12"),
		},
	},
}

var snmpPacketUplink = gosnmp.SnmpPacket{
	Variables: []gosnmp.SnmpPDU{
		{
			Name:  ifDescrUplinkOID,
			Type:  gosnmp.OctetString,
			Value: []byte("xe-0/0/45"),
		},
	},
}

var snmpPacketSysUptime = gosnmp.SnmpPacket{
	Variables: []gosnmp.SnmpPDU{
		{
			Name:  sysUpTimeOID,
			Type:  gosnmp.TimeTicks,
			Value: 1592000258,
		},
	},
}

var snmpPacketMetricsRun1 = gosnmp.SnmpPacket{
	Variables: []gosnmp.SnmpPDU{
		{
			Name:  ifOutDiscardsMachineOID,
			Type:  gosnmp.Counter32,
			Value: uint(0),
		},
		{
			Name:  ifOutDiscardsUplinkOID,
			Type:  gosnmp.Counter32,
			Value: uint(3),
		},
		{
			Name:  ifHCInOctetsMachineOID,
			Type:  gosnmp.Counter64,
			Value: uint64(275),
		},
		{
			Name:  ifHCInOctetsUplinkOID,
			Type:  gosnmp.Counter64,
			Value: uint64(437),
		},
	},
}

var snmpPacketMetricsRun2 = gosnmp.SnmpPacket{
	Variables: []gosnmp.SnmpPDU{
		{
			Name:  ifOutDiscardsMachineOID,
			Type:  gosnmp.Counter32,
			Value: uint(0),
		},
		{
			Name:  ifOutDiscardsUplinkOID,
			Type:  gosnmp.Counter32,
			Value: uint(8),
		},
		{
			Name:  ifHCInOctetsMachineOID,
			Type:  gosnmp.Counter64,
			Value: uint64(511),
		},
		{
			Name:  ifHCInOctetsUplinkOID,
			Type:  gosnmp.Counter64,
			Value: uint64(624),
		},
	},
}

type mockSwitchClient struct {
	err error
	run int
}

func (m *mockSwitchClient) BulkWalkAll(rootOid string) (results []gosnmp.SnmpPDU, err error) {
	return []gosnmp.SnmpPDU{
		{
			Name:  ifDescrMachineOID,
			Type:  gosnmp.OctetString,
			Value: []byte("mlab2"),
		},
		{
			Name:  ifDescrUplinkOID,
			Type:  gosnmp.OctetString,
			Value: []byte("uplink-10g"),
		},
	}, nil
}

func (m *mockSwitchClient) Get(oids []string) (result *gosnmp.SnmpPacket, err error) {
	var packet *gosnmp.SnmpPacket

	// len(oids) will only be one when looking up ifDescr and for test cases.
	if len(oids) == 1 {
		if oids[0] == ifDescrMachineOID {
			packet = &snmpPacketMachine
		}
		if oids[0] == ifDescrUplinkOID {
			packet = &snmpPacketUplink
		}
		if oids[0] == sysUpTimeOID {
			packet = &snmpPacketSysUptime
		}
		if oids[0] == "invalid-oid" {
			packet = nil
		}
	}

	// len(oids) will be greater than one when looking up metrics.
	if len(oids) > 1 {
		if m.run == 1 {
			packet = &snmpPacketMetricsRun1
		}
		if m.run == 2 {
			packet = &snmpPacketMetricsRun2
		}
	}

	return packet, m.err
}

func Test_New(t *testing.T) {
	// Each run of New() registers metrics in a global default prometheus
	// registry. This resets the default registry so that we can run New()
	// serveral times without getting panics about trying to registser the same
	// metric twice.
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	s := &mockSwitchClient{
		err: nil,
	}
	m := New(s, c, target, hostname)

	var expectedMetricsOIDs = map[string]*oid{
		ifOutDiscardsMachineOID: &oid{
			name:          "ifOutDiscards",
			previousValue: 0,
			scope:         "machine",
			ifAlias:       "mlab2",
			ifDescr:       "xe-0/0/12",
			interval: archive.Model{
				Experiment: "s1-abc0t.measurement-lab.org",
				Hostname:   "mlab2-abc0t.mlab-sandbox.measurement-lab.org",
				Metric:     "switch.discards.local.tx",
				Samples:    []archive.Sample{},
			},
		},
		ifOutDiscardsUplinkOID: &oid{
			name:          "ifOutDiscards",
			previousValue: 0,
			scope:         "uplink",
			ifAlias:       "uplink-10g",
			ifDescr:       "xe-0/0/45",
			interval: archive.Model{
				Experiment: "s1-abc0t.measurement-lab.org",
				Hostname:   "mlab2-abc0t.mlab-sandbox.measurement-lab.org",
				Metric:     "switch.discards.uplink.tx",
				Samples:    []archive.Sample{},
			},
		},
		ifHCInOctetsMachineOID: &oid{
			name:          "ifHCInOctets",
			previousValue: 0,
			scope:         "machine",
			ifAlias:       "mlab2",
			ifDescr:       "xe-0/0/12",
			interval: archive.Model{
				Experiment: "s1-abc0t.measurement-lab.org",
				Hostname:   "mlab2-abc0t.mlab-sandbox.measurement-lab.org",
				Metric:     "switch.octets.local.rx",
				Samples:    []archive.Sample{},
			},
		},
		ifHCInOctetsUplinkOID: &oid{
			name:          "ifHCInOctets",
			previousValue: 0,
			scope:         "uplink",
			ifAlias:       "uplink-10g",
			ifDescr:       "xe-0/0/45",
			interval: archive.Model{
				Experiment: "s1-abc0t.measurement-lab.org",
				Hostname:   "mlab2-abc0t.mlab-sandbox.measurement-lab.org",
				Metric:     "switch.octets.uplink.rx",
				Samples:    []archive.Sample{},
			},
		},
	}

	if !reflect.DeepEqual(m.oids, expectedMetricsOIDs) {
		t.Errorf("Unexpected Metrics.oids.\nGot:\n%v\nExpected:\n%v", m.oids, expectedMetricsOIDs)
	}

	if m.hostname != hostname {
		t.Errorf("Unexpected Metrics.hostname.\nGot: %v\nExpected: %v", m.hostname, hostname)
	}

	if m.machine != machine {
		t.Errorf("Unexpected Metrics.machine.\nGot: %v\nExpected: %v", m.machine, machine)
	}

	if !m.firstRun {
		t.Errorf("Metrics.firstRun should be true, but got false.")
	}

}

func Test_Collect(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	var expectedValues = map[string]map[string]uint64{
		ifOutDiscardsMachineOID: map[string]uint64{
			"run1Prev":   0,
			"run2Prev":   0,
			"run2Sample": 0,
		},
		ifOutDiscardsUplinkOID: map[string]uint64{
			"run1Prev":   3,
			"run2Prev":   8,
			"run2Sample": 5,
		},
		ifHCInOctetsMachineOID: map[string]uint64{
			"run1Prev":   275,
			"run2Prev":   511,
			"run2Sample": 236,
		},
		ifHCInOctetsUplinkOID: map[string]uint64{
			"run1Prev":   437,
			"run2Prev":   624,
			"run2Sample": 187,
		},
	}

	s1 := &mockSwitchClient{
		err: nil,
		run: 1,
	}
	m := New(s1, c, target, hostname)
	m.Collect(s1, c)

	for oid := range m.oids {
		// Be sure that previousValues is what we expect.
		if m.oids[oid].previousValue != expectedValues[oid]["run1Prev"] {
			t.Errorf("For OID %v expected a previousValue of %v after run1 but got: %v",
				oid, expectedValues[oid]["run1Prev"], m.oids[oid].previousValue)
		}
		// Be sure that the number of samples is what we expect.
		if len(m.oids[oid].interval.Samples) != 0 {
			t.Errorf("For OID %v expected 0 samples after run1, but got: %v", oid, len(m.oids[oid].interval.Samples))
		}
	}

	s2 := &mockSwitchClient{
		err: nil,
		run: 2,
	}
	m.Collect(s2, c)

	for oid := range m.oids {
		// Be sure that previousValues is what we expect.
		if m.oids[oid].previousValue != expectedValues[oid]["run2Prev"] {
			t.Errorf("For OID %v expected a previousValue of %v after run2 but got: %v",
				oid, expectedValues[oid]["run2Prev"], m.oids[oid].previousValue)
		}
		// Be sure that the number of samples is what we expect.
		if len(m.oids[oid].interval.Samples) != 1 {
			t.Errorf("For OID %v expected 1 samples after run2, but got: %v", oid, len(m.oids[oid].interval.Samples))
		}

		if m.oids[oid].interval.Samples[0].Value != expectedValues[oid]["run2Sample"] {
			t.Errorf("For OID %v expected a sample value of %v after run2 but got: %v",
				oid, expectedValues[oid]["run2Sample"], m.oids[oid].interval.Samples[0].Value)
		}
	}

}

func Test_getOidsInt64BadType(t *testing.T) {
	var s = &mockSwitchClient{}
	var oids = []string{sysUpTimeOID}
	_, err := getOidsInt64(s, oids)
	if err == nil {
		t.Error("Expected an error but didn't get one")
	}
}

func Test_getOidsInt64InvalidOID(t *testing.T) {
	var s = &mockSwitchClient{
		err: fmt.Errorf("ERROR: %v", "marshal: unable to marshal OID: invalid object identifier"),
	}
	var oids = []string{"invalid-oid"}
	_, err := getOidsInt64(s, oids)
	if err == nil {
		t.Errorf("Expected an error but didn't get one")
	}
}

func Test_CollectWithSnmpError(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	s := &mockSwitchClient{}
	m := New(s, c, target, hostname)

	sErr := &mockSwitchClient{
		err: fmt.Errorf("An SNMP error occured: %s", "error"),
		run: 1,
	}
	err := m.Collect(sErr, c)
	if err == nil {
		t.Error("Expected an error but didn't get one")
	}
}

func Test_Write(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	s1 := &mockSwitchClient{
		err: nil,
		run: 1,
	}
	m := New(s1, c, target, hostname)
	m.CollectStart = time.Now()
	m.Collect(s1, c)

	s2 := &mockSwitchClient{
		err: nil,
		run: 2,
	}
	m.Collect(s2, c)

	end := time.Now()
	start := end.Add(time.Duration(10) * -time.Second)
	archivePath := archive.GetPath(start, end, "/tmp/disco", hostname)
	dirPath := path.Dir(archivePath)

	m.Write("/tmp/disco")
	defer os.RemoveAll("/tmp/disco")

	a, err := ioutil.ReadDir(dirPath)
	rtx.Must(err, "Could not read test archive directory")

	if len(a) != 1 {
		t.Errorf("Expected one archive file, but got: %v", len(a))
	}
	os.RemoveAll(fmt.Sprintf("%04d", time.Now().Year()))
}
