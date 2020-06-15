# DISCOv2

DISCOv2 is a reimplementation of the DISCO parts of
[collectd-mlab](https://github.com/m-lab/collectd-mlab), written in Go. It
differs a bit from DISCO, but fundamentally they are the same in that they
both probe a switch for various traffic metrics every 10s and then
periodically write the results to a file in JSON format. Pusher will upload
those files to GCS where they will be processed and parsed into BigQuery.

DISCOv2 supports the following flags:

* `--prometheusx.listen-address`: the IP and TCP port to listen to for
   Prometheus metricis requests.
* `--metrics-file`: the path to a YAML-formatted file defining which metrics to
   scrape. See file metrics.yaml in this repo for an example.
* `--write-interval`: the interval at which collected metrics are converted to
   JSON and written to disk.
* `--target`: the name or IP of the switch to collect metrics from.

DISCOv2 requires that an environment variable named `DISCO_COMMUNITY` is set
and contains the SNMP community sting to use when polling the switch.

Unlike DISCO, in addition to collecting switch metrics every 10s and writing
out data files, DISCOv2 includes a Prometheus exporter which will expose the
metrics it has collected. This makes DISCOv2 something like the
[snmp_exporter](https://github.com/prometheus/snmp_exporter), but far less
general purpose.
