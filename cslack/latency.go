package cslack

import (
	"time"

	"../redis"
	"github.com/golang/glog"
)

// TODO: refactor out to file. All events should be separate files
func handleLatency(latency time.Duration, server *SlackServer) {
	// report only high latency
	var latencyThreshold = time.Duration(2 * time.Second)
	if int64(latency) > int64(latencyThreshold) {
		glog.Errorf("%s latency over threshold(%s): %s", server.Name, latencyThreshold, latency)
	}
	server.LatencySlice = append(server.LatencySlice, latency.Nanoseconds())
	// if *debugCSlack {
	// 	for i, l := range server.LatencySlice {
	// 		glog.Infof("%s latencySlice %d = %d", server.Name, i, l)
	// 	}
	// }
	if server.LatencyCounter == 10 {
		var total int64
		for _, l := range server.LatencySlice {
			total += l
		}
		avg := total / int64(len(server.LatencySlice))
		// save to redis
		now := time.Now()
		jsonNow, err := now.MarshalJSON()
		if err != nil {
			glog.Error("time conversion error: " + err.Error())
		}
		key := server.Name + ":latency:" + string(jsonNow)
		jsonLatency := string(avg)
		redis.Set(key, []byte(jsonLatency))
		glog.Infof("%s avg latency now %s", server.Name, time.Duration(avg))
		server.LatencyCounter = 0
	} else {
		if *debugCSlack && *debugLatencyTick {
			glog.Infof("%s Tick %d", server.Name, server.LatencyCounter)
		}
		server.LatencyCounter++
	}
	glog.Flush()
}

// func reportLatency(server SlackServer) {

// }
