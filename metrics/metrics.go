package metrics

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/m-lab/disco/archive"
	"github.com/m-lab/disco/config"
	"github.com/m-lab/disco/snmp"
	"github.com/m-lab/go/rtx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	ifAliasOid     = ".1.3.6.1.2.1.31.1.1.1.18"
	ifDescrOidStub = ".1.3.6.1.2.1.2.2.1.2"
)

// Metrics represents a collection of oids, plus additional data about the environment.
type Metrics struct {
	// TODO(kinkade): remove this field in favor of a more elegant solution.
	firstRun      bool
	hostname      string
	oids          map[string]oid
	machine       string
	mutex         sync.Mutex
	prom          map[string]*prometheus.CounterVec
	CollectStart  time.Time
	IntervalStart time.Time
}

type oid struct {
	name           string
	previousValue  uint64
	scope          string
	ifDescr        string
	intervalSeries archive.Model
}

// getIfaces uses an ifAlias value to determine the logical interface number and
// description for the machine's interface and the switch's uplink.
func getIfaces(snmp snmp.SNMP, machine string) map[string]map[string]string {
	pdus, err := snmp.BulkWalkAll(ifAliasOid)
	rtx.Must(err, "Failed to walk the ifAlias OID")

	ifaces := map[string]map[string]string{
		"machine": map[string]string{
			"iface":   "",
			"ifDescr": "",
		},
		"uplink": map[string]string{
			"iface":   "",
			"ifDescr": "",
		},
	}

	for _, pdu := range pdus {
		oidParts := strings.Split(pdu.Name, ".")
		iface := oidParts[len(oidParts)-1]

		b := pdu.Value.([]byte)
		val := strings.TrimSpace(string(b))
		if val == machine {
			ifDescrOid := createOID(ifDescrOidStub, iface)
			oidMap, err := getOidsString(snmp, []string{ifDescrOid})
			rtx.Must(err, "Failed to determine the machine interface ifDescr")
			ifaces["machine"]["ifDescr"] = oidMap[ifDescrOid]
			ifaces["machine"]["iface"] = iface
		}
		if strings.HasPrefix(val, "uplink") {
			ifDescrOid := createOID(ifDescrOidStub, iface)
			oidMap, err := getOidsString(snmp, []string{ifDescrOid})
			rtx.Must(err, "Failed to determine the uplink interface ifDescr")
			ifaces["uplink"]["ifDescr"] = oidMap[ifDescrOid]
			ifaces["uplink"]["iface"] = iface
		}
	}

	return ifaces
}

// getOidsString accepts a list of OIDS and returns a map of the OIDs to their
// string values.
func getOidsString(snmp snmp.SNMP, oids []string) (map[string]string, error) {
	oidMap := make(map[string]string)
	result, err := snmp.Get(oids)
	for _, pdu := range result.Variables {
		oidMap[pdu.Name] = string(pdu.Value.([]byte))
	}
	return oidMap, err
}

// getOidsInt64 accepts a list of OIDS and returns a map of the OIDs to their
// various int-type values, with all values being cast to a uint64.
//
// Counter32 OIDs seem to be presented as type uint, while Counter64 OIDs seem
// to be presented as type uint64.
func getOidsInt64(snmp snmp.SNMP, oids []string) (map[string]uint64, error) {
	oidMap := make(map[string]uint64)
	result, err := snmp.Get(oids)
	if result == nil {
		err = fmt.Errorf("No results returned from server for oids: %v", oids)
		return nil, err
	}
	for _, pdu := range result.Variables {
		switch value := pdu.Value.(type) {
		case uint:
			oidMap[pdu.Name] = uint64(value)
		case uint64:
			oidMap[pdu.Name] = value
		default:
			err = fmt.Errorf("Unknown type %T of SNMP type %v for OID %v", value, pdu.Type, pdu.Name)
			return nil, err
		}
	}
	return oidMap, err
}

// createOID joins an OID stub with a logical interface number, returning the
// complete OID.
func createOID(oidStub string, iface string) string {
	return fmt.Sprintf("%v.%v", oidStub, iface)
}

// Collect scrapes values for a list of OIDs and updates a map of OIDs,
// appending a new archive.Sample representing the increase from the previous
// scrape to an array of samples for that OID.
func (metrics *Metrics) Collect(snmp snmp.SNMP, config config.Config) error {
	// Set a lock to avoid a race between the collecting and writing of metrics.
	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()

	oids := []string{}
	for oid := range metrics.oids {
		oids = append(oids, oid)
	}

	collectStart := time.Now()
	oidValueMap, err := getOidsInt64(snmp, oids)
	if err != nil {
		log.Printf("ERROR: failed to GET OIDs (%v) from SNMP server: %v", oids, err)
		// TODO(kinkade): increment some sort of error metric here.
		return err
	}
	collectEnd := time.Now()

	for oid, value := range oidValueMap {
		// This is less than ideal. Because we can't write to a map in a struct
		// we have to copy the whole map, modify it and then overwrite the
		// original map. There is likely a better way to do this.
		metricOid := metrics.oids[oid]
		metricOid.previousValue = value

		// If this is the first run then we have no previousValue with which to
		// calculate an increase, so we just record a previousValue and return.
		if metrics.firstRun == true {
			metrics.oids[oid] = metricOid
			continue
		}

		increase := value - metrics.oids[oid].previousValue
		ifDescr := metrics.oids[oid].ifDescr
		metricName := metrics.oids[oid].name
		metrics.prom[metricName].WithLabelValues(metrics.hostname, ifDescr).Add(float64(increase))

		metricOid.intervalSeries.Samples = append(
			metricOid.intervalSeries.Samples,
			archive.Sample{
				Timestamp:    metrics.CollectStart.Unix(),
				CollectStart: collectStart.UnixNano(),
				CollectEnd:   collectEnd.UnixNano(),
				Value:        increase},
		)
		metrics.oids[oid] = metricOid
	}

	if metrics.firstRun == true {
		metrics.firstRun = false
	}

	return nil
}

// Write collects JSON data for all OIDs and then writes the result to an archive.
func (metrics *Metrics) Write(start time.Time, end time.Time) {
	var jsonData []byte

	// Set a lock to avoid a race between the collecting and writing of metrics.
	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()

	for oid, values := range metrics.oids {
		data := archive.MustMarshalJSON(values.intervalSeries)
		jsonData = append(jsonData, data...)

		// This is less than ideal. Because we can't write to a map in a struct
		// we have to copy the whole map, modify it and then overwrite the
		// original map. There is likely a better way to do this.
		metricsOid := metrics.oids[oid]
		metricsOid.intervalSeries.Samples = []archive.Sample{}
		metrics.oids[oid] = metricsOid
	}

	archivePath := archive.GetPath(start, end, metrics.hostname)
	err := archive.Write(archivePath, jsonData)
	if err != nil {
		rtx.Must(err, "Failed to write archive")
	}
	metrics.IntervalStart = time.Now()
}

// New creates a new metrics.Metrics struct with various OID maps initialized.
func New(snmp snmp.SNMP, config config.Config, target string, hostname string) *Metrics {
	machine := hostname[:5]
	ifaces := getIfaces(snmp, machine)

	m := &Metrics{
		oids:     make(map[string]oid),
		prom:     make(map[string]*prometheus.CounterVec),
		hostname: hostname,
		machine:  machine,
		firstRun: true,
	}

	for _, metric := range config.Metrics {
		discoNames := map[string]string{
			"machine": metric.MlabMachineName,
			"uplink":  metric.MlabUplinkName,
		}
		for scope, values := range ifaces {
			oidStr := createOID(metric.OidStub, values["iface"])
			o := oid{
				name:    metric.Name,
				scope:   scope,
				ifDescr: values["ifDescr"],
				intervalSeries: archive.Model{
					Experiment: target,
					Hostname:   hostname,
					Metric:     discoNames[scope],
					Samples:    []archive.Sample{},
				},
			}
			m.oids[oidStr] = o
		}
		m.prom[metric.Name] = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: metric.Name,
				Help: metric.Description,
			},
			[]string{
				"node",
				"interface",
			},
		)
	}

	return m
}
