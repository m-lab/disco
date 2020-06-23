package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/m-lab/disco/config"
	"github.com/m-lab/disco/metrics"
	"github.com/m-lab/disco/snmp"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/prometheusx"
	"github.com/m-lab/go/rtx"
	"github.com/soniah/gosnmp"
)

var (
	fCommunity          = flag.String("community", "", "The SNMP community string for the switch.")
	fDataDir            = flag.String("datadir", "/var/spool/disco", "Base directory where metrics files will be written.")
	fHostname           = flag.String("hostname", "", "The FQDN of the node.")
	fMetricsFile        = flag.String("metrics", "", "Path to YAML file defining metrics to scrape.")
	fWriteInterval      = flag.Duration("write-interval", 300*time.Second, "Interval to write out JSON files e.g, 300s, 10m.")
	fTarget             = flag.String("target", "", "Switch FQDN to scrape metrics from.")
	mainCtx, mainCancel = context.WithCancel(context.Background())
)

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnv(flag.CommandLine), "Could not parse env args")

	if len(*fCommunity) <= 0 {
		log.Fatal("SNMP community string must be passed as arg or env variable.")
	}

	if len(*fHostname) <= 0 {
		log.Fatal("Node's FQDN must be passed as an arg or env variable.")
	}

	// If the -target flag is empty, then attempt to construct it using the hostname.
	if len(*fTarget) <= 0 {
		h := *fHostname
		*fTarget = fmt.Sprintf("s1-%s.measurement-lab.org", h[6:11])
	}

	goSNMP := &gosnmp.GoSNMP{
		Target:    *fTarget,
		Port:      uint16(161),
		Community: strings.TrimSpace(*fCommunity),
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(5) * time.Second,
		Retries:   1,
	}
	err := goSNMP.Connect()
	rtx.Must(err, "Failed to connect to the SNMP server")

	config, err := config.New(*fMetricsFile)
	rtx.Must(err, "Could not create new metrics configuration")
	client := snmp.New(goSNMP)
	metrics := metrics.New(client, config, *fTarget, *fHostname)

	promSrv := prometheusx.MustServeMetrics()

	go func() {
		<-mainCtx.Done()
		goSNMP.Conn.Close()
		promSrv.Close()
	}()

	// Start scraping on a clean 10s boundary within a minute. Run in an very
	// tight loop to be sure we start things as early in the 10s boundary as
	// possible.
	for time.Now().Second()%10 != 0 {
		time.Sleep(1 * time.Millisecond)
	}

	writeTicker := time.NewTicker(*fWriteInterval)
	metrics.IntervalStart = time.Now()

	collectTicker := time.NewTicker(10 * time.Second)
	// Tickers wait for the configured duration before their first tick. We want
	// Collect() to run immedately, so manually kick off Collect() once
	// immediately after the ticker is created.
	metrics.Collect(client, config)

	for {
		select {
		case <-mainCtx.Done():
			collectTicker.Stop()
			writeTicker.Stop()
			return
		case <-writeTicker.C:
			start := metrics.IntervalStart
			metrics.IntervalStart = time.Now()
			metrics.Write(start, time.Now(), *fDataDir)
		case <-collectTicker.C:
			metrics.CollectStart = time.Now()
			metrics.Collect(client, config)
		}
	}
}
