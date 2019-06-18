claymore-stats-influxdb-go
================

What does it do?
----------------

Reads statistics from Claymore and sends them to an InfluxDB instance at defined intervals. 

Stats include overall hashrate, per GPU hashrate, per GPU temperature, shares, and uptime

Usage
-----

Available flags:

```cgo
$ ./claymore-stats-influxdb-go
Usage of ./claymore-stats-influxdb-go
  -claymore-addr string
    	address and port of the claymore computer (default "127.0.0.1")
  -http-port string
    	port for the http server to listen on for health checks (default "8085")
  -influxdb-addr string
    	address of influxdb endpoint, ex: http://127.0.0.1:8086 (default "http://127.0.0.1:8086")
  -influxdb-database string
    	influxdb database to store datapoints (default "homelab_custom")
  -influxdb-pass string
    	password for influxdb access (default "admin")
  -influxdb-user string
    	username for influxdb access (default "admin")

```

To Do
------
- Look, I'm aware this code is terrible, it needs to be cleaned up
- Add real error handling
