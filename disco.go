package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/m-lab/disco/config"
	"github.com/m-lab/disco/snmp"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/prometheusx"
	"github.com/m-lab/go/rtx"
	"github.com/nkinkade/disco-go/metrics"
	"github.com/soniah/gosnmp"
)

var (
	fCommunity          = flag.String("community", "", "The SNMP community string for the switch.")
	fHostname           = flag.String("hostname", "", "The FQDN of the node.")
	fListenAddress      = flag.String("listen-address", ":8888", "Address to listen on for telemetry.")
	fMetricsFile        = flag.String("metrics", "", "Path to YAML file defining metrics to scrape.")
	fWriteInterval      = flag.Uint64("write-interval", 300, "Interval in seconds to write out JSON files.")
	fTarget             = flag.String("target", "", "Switch FQDN to scrape metrics from.")
	logFatal            = log.Fatal
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

	goSNMP := &gosnmp.GoSNMP{
		Target:    *fTarget,
		Port:      uint16(161),
		Community: *fCommunity,
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

	// Start scraping on a clean 10s boundary within a minute.
	for time.Now().Second()%10 != 0 {
		time.Sleep(1 * time.Second)
	}

	promSrv := prometheusx.MustServeMetrics()

	go func() {
		<-mainCtx.Done()
		goSNMP.Conn.Close()
		promSrv.Close()
	}()

	cronWriteMetrics := gocron.NewScheduler(time.UTC)
	cronWriteMetrics.Every(*fWriteInterval).Seconds().Do(metrics.Write, *fWriteInterval)
	cronWriteMetrics.StartAsync()

	cronCollectMetrics := gocron.NewScheduler(time.UTC)
	cronCollectMetrics.Every(10).Seconds().StartImmediately().Do(metrics.Collect, client, config)
	cronCollectMetrics.StartBlocking()
}
