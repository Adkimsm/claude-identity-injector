package main

import (
	"sync/atomic"
	"time"
)

var processStart = time.Now()

type metricSnapshot struct {
	seen          uint64
	matched       uint64
	injected      uint64
	unmatched     uint64
	errors        uint64
	repairedTools uint64
	uptime        int64
}

var metricState struct {
	seen          atomic.Uint64
	matched       atomic.Uint64
	injected      atomic.Uint64
	unmatched     atomic.Uint64
	errors        atomic.Uint64
	repairedTools atomic.Uint64
}

func recordRequestMetric(req *requestInterceptRequest, kind string) {
	_ = req
	metricState.seen.Add(1)
	switch kind {
	case "unmatched":
		metricState.unmatched.Add(1)
	case "injected":
		metricState.injected.Add(1)
	case "error":
		metricState.errors.Add(1)
	}
	if kind == "injected" || kind == "unmatched" {
		metricState.matched.Add(1)
	}
}

func recordToolRepair(req *streamChunkInterceptRequest) {
	_ = req
	metricState.repairedTools.Add(1)
}

func clearRequestMetrics() {
	metricState.seen.Store(0)
	metricState.matched.Store(0)
	metricState.injected.Store(0)
	metricState.unmatched.Store(0)
	metricState.errors.Store(0)
	metricState.repairedTools.Store(0)
}

func currentMetrics() metricSnapshot {
	return metricSnapshot{
		seen:          metricState.seen.Load(),
		matched:       metricState.matched.Load(),
		injected:      metricState.injected.Load(),
		unmatched:     metricState.unmatched.Load(),
		errors:        metricState.errors.Load(),
		repairedTools: metricState.repairedTools.Load(),
		uptime:        int64(time.Since(processStart).Seconds()),
	}
}