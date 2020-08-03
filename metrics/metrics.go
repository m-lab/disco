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

var (
	collectDuration *prometheus.HistogramVec
	collectErrors   *prometheus.CounterVec
)

// Metrics represents a collection of oids, plus additional data about the environment.
type Metrics struct {
	// TODO(kinkade): remove this field in favor of a more elegant solution.
	firstRun      bool
	hostname      string
	oids          map[string]*oid
	machine       string
	mutex         sync.Mutex
	prom          map[string]*prometheus.CounterVec
	CollectStart  time.Time
	IntervalStart time.Time
}

type oid struct {
	name          string
	previousValue uint64
	scope         string
	ifDescr       string
	interval      archive.Model
}

// mustGetIfaces uses an ifAlias value to determine the logical interface number and
// description for the machine's interface and the switch's uplink.
func mustGetIfaces(client snmp.Client, machine string) map[string]map[string]string {
	pdus, err := client.BulkWalkAll(ifAliasOid)
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
			oidMap, err := getOidsString(client, []string{ifDescrOid})
			rtx.Must(err, "Failed to determine the machine interface ifDescr")
			ifaces["machine"]["ifDescr"] = oidMap[ifDescrOid]
			ifaces["machine"]["iface"] = iface
		}
		if strings.HasPrefix(val, "uplink") {
			ifDescrOid := createOID(ifDescrOidStub, iface)
			oidMap, err := getOidsString(client, []string{ifDescrOid})
			rtx.Must(err, "Failed to determine the uplink interface ifDescr")
			ifaces["uplink"]["ifDescr"] = oidMap[ifDescrOid]
			ifaces["uplink"]["iface"] = iface
		}
	}

	// Fail if any machine information was not found.
	if ifaces["machine"]["iface"] == "" {
		log.Fatalf("Failed to find logical iface number for machine: %v", machine)
	}
	if ifaces["machine"]["ifDescr"] == "" {
		log.Fatalf("Failed to find ifDescr for machine logical iface: %v", ifaces["machine"]["iface"])
	}

	// Fail if any uplink information was not found.
	if ifaces["uplink"]["iface"] == "" {
		log.Fatal("Failed to find logical iface number for uplink")
	}
	if ifaces["uplink"]["ifDescr"] == "" {
		log.Fatalf("Failed to find ifDescr for uplink logical iface: %v", ifaces["uplink"]["iface"])
	}

	return ifaces
}

// getOidsString accepts a list of OIDS and returns a map of the OIDs to their
// string values.
func getOidsString(client snmp.Client, oids []string) (map[string]string, error) {
	oidMap := make(map[string]string)
	result, err := client.Get(oids)
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
func getOidsInt64(client snmp.Client, oids []string) (map[string]uint64, error) {
	oidMap := make(map[string]uint64)
	result, err := client.Get(oids)
	if err != nil {
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
	return oidMap, nil
}

// createOID joins an OID stub with a logical interface number, returning the
// complete OID.
func createOID(oidStub string, iface string) string {
	return fmt.Sprintf("%v.%v", oidStub, iface)
}

// Collect scrapes values for a list of OIDs and updates a map of OIDs,
// appending a new archive.Sample representing the increase from the previous
// scrape to an slice of samples for that OID.
func (metrics *Metrics) Collect(client snmp.Client, config config.Config) error {
	// Set a lock to avoid a race between the collecting and writing of metrics.
	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()

	oids := []string{}
	for oid := range metrics.oids {
		oids = append(oids, oid)
	}

	collectStart := time.Now()
	oidValueMap, err := getOidsInt64(client, oids)
	if err != nil {
		log.Printf("ERROR: failed to GET OIDs (%v) from SNMP server: %v", oids, err)
		collectErrors.WithLabelValues(metrics.hostname).Inc()
		return err
	}
	collectEnd := time.Now()

	// Add the collect duration in seconds to a historgram metric.
	collectDuration.WithLabelValues(metrics.hostname).Observe(
		float64(collectEnd.Sub(collectStart)) / float64(time.Second),
	)

	for oid, value := range oidValueMap {
		// If this is the first run then we have no previousValue with which to
		// calculate an increase, so we just record a previousValue and return.
		if metrics.firstRun {
			metrics.oids[oid].previousValue = value
			continue
		}

		increase := value - metrics.oids[oid].previousValue
		ifDescr := metrics.oids[oid].ifDescr
		metricName := metrics.oids[oid].name
		metrics.prom[metricName].WithLabelValues(ifDescr).Add(float64(increase))

		metrics.oids[oid].interval.Samples = append(
			metrics.oids[oid].interval.Samples,
			archive.Sample{
				Timestamp:    metrics.CollectStart.Unix(),
				CollectStart: collectStart.UnixNano(),
				CollectEnd:   collectEnd.UnixNano(),
				Value:        increase},
		)

		metrics.oids[oid].previousValue = value
	}

	if metrics.firstRun {
		metrics.firstRun = false
	}

	return nil
}

// Write collects JSON data for all OIDs and then writes the result to an archive.
func (metrics *Metrics) Write(start time.Time, dataDir string) {
	var jsonData []byte
	var endTimeUnix int64

	// Set a lock to avoid a race between the collecting and writing of metrics.
	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()

	for oid, values := range metrics.oids {
		data := archive.MustMarshalJSON(values.interval)
		jsonData = append(jsonData, data...)
		// Adds a newline to the end of the JSON data to effectively create JSONL.
		jsonData = append(jsonData, '\n')
		// Capture the value of the final Unix timestamp of each sample set. We
		// will use this value to calculate the text of the end time for the
		// file being written. There is an inefficiency here. This variable will
		// get written on every loop iteration, but we will actually only use
		// the last value assigned to it. The final timestamp for every metric
		// should be the same. This seemed better than any alternative trying to
		// determine the last element in the range and only assigning once.
		//
		// NOTE: This usage is selecting the final timestamp of an arbitrary map
		// element, which works because every timestamp for a given collection
		// is necessarily the same. But please note that if this changes in the
		// future that this method will no longer be reliable.
		endTimeUnix = metrics.oids[oid].interval.Samples[len(metrics.oids[oid].interval.Samples)-1].Timestamp
		// Reset the samples to an empty slice of archive.Sample for the next
		// interval.
		metrics.oids[oid].interval.Samples = []archive.Sample{}
	}

	end := time.Unix(endTimeUnix, 0)
	archivePath := archive.GetPath(start, end, dataDir, metrics.hostname)
	err := archive.Write(archivePath, jsonData)
	if err != nil {
		rtx.Must(err, "Failed to write archive")
	}
	metrics.IntervalStart = time.Now()
}

// New creates a new metrics.Metrics struct with various OID maps initialized.
func New(client snmp.Client, config config.Config, target string, hostname string) *Metrics {
	machine := hostname[:5]
	ifaces := mustGetIfaces(client, machine)

	collectDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "disco_collect_duration_seconds",
			Help:    "SNMP collection duration distribution.",
			Buckets: []float64{0.1, 0.3, 0.5, 1, 3, 5},
		},
		[]string{"machine"},
	)

	collectErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "disco_collect_errors_total",
			Help: "Total number SNMP collection errors.",
		},
		[]string{"machine"},
	)

	m := &Metrics{
		firstRun: true,
		hostname: hostname,
		machine:  machine,
		oids:     make(map[string]*oid),
		prom:     make(map[string]*prometheus.CounterVec),
	}

	for _, metric := range config.Metrics {
		discoNames := map[string]string{
			"machine": metric.MlabMachineName,
			"uplink":  metric.MlabUplinkName,
		}
		for scope, values := range ifaces {
			oidStr := createOID(metric.OidStub, values["iface"])
			o := &oid{
				name:    metric.Name,
				scope:   scope,
				ifDescr: values["ifDescr"],
				interval: archive.Model{
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
				"interface",
			},
		)
	}

	return m
}
