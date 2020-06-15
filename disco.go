package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/m-lab/go/prometheusx"
	"github.com/m-lab/go/rtx"
	"github.com/nkinkade/disco-go/config"
	"github.com/nkinkade/disco-go/metrics"
	"github.com/nkinkade/disco-go/snmp"
	"github.com/soniah/gosnmp"
)

var (
	community           = os.Getenv("DISCO_COMMUNITY")
	fListenAddress      = flag.String("listen-address", ":8888", "Address to listen on for telemetry.")
	fMetricsFile        = flag.String("metrics", "", "Path to YAML file defining metrics to scrape.")
	fWriteInterval      = flag.Uint64("write-interval", 300, "Interval in seconds to write out JSON files.")
	fTarget             = flag.String("target", "", "Switch FQDN to scrape metrics from.")
	logFatal            = log.Fatal
	mainCtx, mainCancel = context.WithCancel(context.Background())
)

func main() {
	flag.Parse()

	if len(community) <= 0 {
		log.Fatalf("Environment variable not set: DISCO_COMMUNITY")
	}

	hostname, err := os.Hostname()
	rtx.Must(err, "Failed to determine the hostname of the system")

	goSNMP := &gosnmp.GoSNMP{
		Target:    *fTarget,
		Port:      uint16(161),
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(2) * time.Second,
		Retries:   1,
	}
	err = goSNMP.Connect()
	rtx.Must(err, "Failed to connect to the SNMP server")

	config, err := config.New(*fMetricsFile)
	rtx.Must(err, "Could not create new metrics configuration")
	client := snmp.Client(goSNMP)
	metrics := metrics.New(client, config, *fTarget, hostname)

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
