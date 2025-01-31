package metrics

import (
	"time"

	"github.com/gravitl/netmaker/logger"
	proxy_metrics "github.com/gravitl/netmaker/metrics"
	"github.com/gravitl/netmaker/models"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// Collect - collects metrics
func Collect(iface, server, network string, peerMap models.PeerMap, proxy bool) (*models.Metrics, error) {
	var metrics models.Metrics
	metrics.Connectivity = make(map[string]models.Metric)
	var wgclient, err = wgctrl.New()
	if err != nil {
		fillUnconnectedData(&metrics, peerMap)
		return &metrics, err
	}
	defer wgclient.Close()
	device, err := wgclient.Device(iface)
	if err != nil {
		fillUnconnectedData(&metrics, peerMap)
		return &metrics, err
	}
	// TODO handle freebsd??
	for i := range device.Peers {
		currPeer := device.Peers[i]
		if _, ok := peerMap[currPeer.PublicKey.String()]; !ok {
			continue
		}
		id := peerMap[currPeer.PublicKey.String()].ID
		address := peerMap[currPeer.PublicKey.String()].Address
		if id == "" || address == "" {
			logger.Log(0, "attempted to parse metrics for invalid peer from server", id, address)
			continue
		}
		proxyMetrics := proxy_metrics.GetMetric(server, currPeer.PublicKey.String())
		var newMetric = models.Metric{
			NodeName: peerMap[currPeer.PublicKey.String()].Name,
		}
		logger.Log(2, "collecting metrics for peer", address)
		newMetric.TotalReceived = int64(proxyMetrics.TrafficRecieved)
		newMetric.TotalSent = int64(proxyMetrics.TrafficSent)
		newMetric.Latency = int64(proxyMetrics.LastRecordedLatency)
		newMetric.Connected = proxyMetrics.NodeConnectionStatus[id]
		newMetric.CollectedByProxy = proxy
		if newMetric.Connected {
			newMetric.Uptime = 1
		}
		// check device peer to see if WG is working if ping failed
		if !newMetric.Connected {
			if currPeer.ReceiveBytes > 0 &&
				currPeer.TransmitBytes > 0 &&
				time.Now().Before(currPeer.LastHandshakeTime.Add(time.Minute<<1)) {
				newMetric.Connected = true
				newMetric.Uptime = 1
			}
		}
		newMetric.TotalTime = 1
		metrics.Connectivity[id] = newMetric
		if len(proxyMetrics.NodeConnectionStatus) == 1 {
			proxy_metrics.ResetMetricsForPeer(server, currPeer.PublicKey.String())
		} else {
			proxy_metrics.ResetMetricForNode(server, currPeer.PublicKey.String(), id)
		}
	}

	fillUnconnectedData(&metrics, peerMap)
	return &metrics, nil
}

// == used to fill zero value data for non connected peers ==
func fillUnconnectedData(metrics *models.Metrics, peerMap models.PeerMap) {
	for r := range peerMap {
		id := peerMap[r].ID
		if !metrics.Connectivity[id].Connected {
			newMetric := models.Metric{
				NodeName:  peerMap[r].Name,
				Uptime:    0,
				TotalTime: 1,
				Connected: false,
				Latency:   999,
				PercentUp: 0,
			}
			metrics.Connectivity[id] = newMetric
		}
	}
}
