package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/influxdata/influxdb/client/v2"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// The Results array is indexed like so:
//   0: version
//   1: uptime (minutes)
//   2: hashrate; shares; rejected_shares
//   3: hashrate
//   4: DCR hashrate; shares; rejected_shares
//   5: DCR hashrate for all GPUs
//   6: temperature and fan speed for all GPUs
//   7: mining pool
//   8: number of ETH invalid shares, number of ETH pool switches, number of DCR invalid shares,
//      number of DCR pool switches.

type ResponseJson struct {
	Results []string `json:"result"`
	Id      int      `json:"id"`
	Error   string   `json:"error"`
}

type BaseConfig struct {
	CheckInterval  int
	ClaymoreAddr   string
	HealthHttpPort string
	InfluxDatabase string
	InfluxAddr     string
	InfluxUser     string
	InfluxPass     string
}

var (
	hostname, _ = os.Hostname()
)

func HealthHandler(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("OK"))
}

func safeStringToFloat(s string) (float64, bool) {
	if conv, err := strconv.ParseFloat(s, 32); err == nil {
		return conv, true
	}

	return 0, false
}

func pollClaymoreApi(bc *BaseConfig) {
	for {
		var response ResponseJson

	    conn, _ := net.Dial("tcp", bc.ClaymoreAddr)
	    req := "{\"id\":0,\"jsonrpc\":\"2.0\",\"method\":\"miner_getstat1\"}"

	    // send the request to the socket
	    fmt.Fprintf(conn, req + "\n")

	    // read the response
	    message, _ := bufio.NewReader(conn).ReadString('\n')
	    json.Unmarshal([]byte(message), &response)

	    // close the connection
	    conn.Close()

		c, err := client.NewHTTPClient(client.HTTPConfig{
			Addr: bc.InfluxAddr,
			Username: bc.InfluxUser,
			Password: bc.InfluxPass,
		})

		if err != nil {
			log.Fatal(err)
		}


		bp, err := client.NewBatchPoints(client.BatchPointsConfig{
			Database: bc.InfluxDatabase,
			Precision: "s",
		})

		if err != nil {
			log.Fatal(err)
		}

		tags := map[string]string {
			"host": hostname,
		}

		curTime := time.Now()

		// The Results array is indexed like so:
		//   0: version
		//   1: uptime (minutes)
		//   2: hashrate; shares; rejected_shares
		//   3: hashrate
		//   4: DCR hashrate; shares; rejected_shares
		//   5: DCR hashrate for all GPUs
		//   6: temperature and fan speed for all GPUs
		//   7: mining pool
		//   8: number of ETH invalid shares, number of ETH pool switches, number of DCR invalid shares,
		//      number of DCR pool switches.

		// uptime
		if tmpVal, success := safeStringToFloat(response.Results[1]); success {
			pt, err := client.NewPoint("claymore_stats", tags, map[string]interface{}{
				"uptime": tmpVal,
			}, curTime)

			if err != nil {
				log.Fatal(err)
			}

			bp.AddPoint(pt)
		}

		// lets split the string at semicolon
		hrSharesRejectedSharesArr := strings.Split(response.Results[2], ";")

		// shares
		if tmpVal, success := safeStringToFloat(hrSharesRejectedSharesArr[1]); success {
			pt, err := client.NewPoint("claymore_stats", tags, map[string]interface{}{
				"shares": tmpVal,
			}, curTime)

			if err != nil {
				log.Fatal(err)
			}

			bp.AddPoint(pt)
		}

		// total hashrate
		if tmpVal, success := safeStringToFloat(hrSharesRejectedSharesArr[0]); success {
			modifiedResult := tmpVal / 1000
			pt, err := client.NewPoint("claymore_stats", tags, map[string]interface{}{
				"hashrate": modifiedResult,
			}, curTime)

			if err != nil {
				log.Fatal(err)
			}

			bp.AddPoint(pt)
		}

		// split the hashrate by gpu string at ;
		hrByGpuArr := strings.Split(response.Results[3], ";")

		// hashrate by gpu
		for i, d := range hrByGpuArr {
			if tmpVal, success := safeStringToFloat(d); success {
		    	modifiedResult := tmpVal / 1000
		    	pt, err := client.NewPoint("claymore_stats", tags, map[string]interface{}{
		    		"gpu_"+strconv.Itoa(i)+"_hashrate": modifiedResult,
		    	}, curTime)

		    	if err != nil {
		    		log.Fatal(err)
		    	}

				bp.AddPoint(pt)
			}
		}

		tmpFanSpeedByGpuArr := strings.Split(response.Results[6], ";")

		for i, d := range tmpFanSpeedByGpuArr {
			if i % 2 == 0 {
				// even, temperature
				if tmpVal, success := safeStringToFloat(d); success {
					pt, err := client.NewPoint("claymore_stats", tags, map[string]interface{}{
						"gpu_"+strconv.Itoa(i)+"_temperature": tmpVal,
					}, curTime)

					if err != nil {
						log.Fatal(err)
					}

					bp.AddPoint(pt)

					fahrenheit := (tmpVal * 9/5) + 32

					pt, err = client.NewPoint("claymore_stats", tags, map[string]interface{}{
						"gpu_"+strconv.Itoa(i)+"_temperature_f": fahrenheit,
					}, curTime)

					if err != nil {
						log.Fatal(err)
					}

					bp.AddPoint(pt)
				}
			} else {
				// odd, fan speed
				if tmpVal, success := safeStringToFloat(d); success {
					pt, err := client.NewPoint("claymore_stats", tags, map[string]interface{}{
						"gpu_"+strconv.Itoa(i - 1)+"_fan_speed": tmpVal,
					}, curTime)

					if err != nil {
						log.Fatal(err)
					}

					bp.AddPoint(pt)
				}
			}
		}

		if err := c.Write(bp); err != nil {
			log.Fatal(err)
		}

		if err := c.Close(); err != nil {
			log.Fatal(err)
		}

		// cleanup http and influx clients
		c.Close()

		log.Print("Points submitted to influxdb...")
		time.Sleep(20 * time.Second)
	}
}

func main() {
	bc := new(BaseConfig)
	flag.StringVar(&bc.ClaymoreAddr,"claymore-addr", "127.0.0.1:3333", "address and port of the claymore host")
	flag.StringVar(&bc.HealthHttpPort,"http-port", "8085", "port for the http server to listen on for health checks")
	flag.StringVar(&bc.InfluxDatabase,"influxdb-database", "homelab_custom", "influxdb database to store datapoints")
	flag.StringVar(&bc.InfluxAddr,"influxdb-addr", "http://127.0.0.1:8086", "address of influxdb endpoint, ex: http://127.0.0.1:8086")
	flag.StringVar(&bc.InfluxUser,"influxdb-user", "admin", "username for influxdb access")
	flag.StringVar(&bc.InfluxPass,"influxdb-pass", "admin", "password for influxdb access")
	flag.IntVar(&bc.CheckInterval, "check-interval", 20, "frequency to poll the claymore endpoint")

	flag.Parse()

	go pollClaymoreApi(bc)

	r := mux.NewRouter()
	r.HandleFunc("/healthz", HealthHandler)
	log.Printf("Listening on :%s", bc.HealthHttpPort)

	log.Fatal(http.ListenAndServe(":" + bc.HealthHttpPort, r))
}