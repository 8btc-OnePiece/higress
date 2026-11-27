package main

import (
	"net/http"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

// Reserve before upstream execution. A slow receiver applies backpressure to new
// paid requests rather than evicting events that already represent consumption.
const maxPendingTeamReports = 1024

type teamDelivery struct {
	cluster *wrapper.FQDNCluster
	timeout uint32
	pending map[*teamEvent]struct{}
	tick    uint64
}

type teamEvent struct {
	id      string
	body    []byte
	sending bool
	next    uint64
	delay   uint64
}

func (q *teamDelivery) reserve(id string) *teamEvent {
	if len(q.pending) >= maxPendingTeamReports {
		return nil
	}
	e := &teamEvent{id: id}
	q.pending[e] = struct{}{}
	return e
}

// Run callouts on the plugin context. The SDK discards callbacks owned by an
// HTTP stream once its client disconnects, including successful acknowledgements.
func (q *teamDelivery) flush() {
	q.tick++
	sent := 0
	for e := range q.pending {
		if len(e.body) == 0 || e.sending || q.tick < e.next {
			continue
		}
		e.sending = true
		_, err := proxywasm.DispatchHttpCall(q.cluster.ClusterName(), [][2]string{
			{":method", http.MethodPost}, {":authority", q.cluster.HostName()},
			{":path", "/internal/model/usage/team-report"}, {"content-type", "application/json"},
		}, e.body, nil, q.timeout, func(_, size, _ int) {
			headers, _ := proxywasm.GetHttpCallResponseHeaders()
			for _, h := range headers {
				if h[0] == ":status" && h[1] == "200" {
					reply, err := proxywasm.GetHttpCallResponseBody(0, size)
					if err == nil && gjson.GetBytes(reply, "data.success").Type == gjson.True &&
						gjson.GetBytes(reply, "data.requestId").String() == e.id {
						delete(q.pending, e)
						return
					}
				}
			}
			e.retry(q.tick)
		})
		if err != nil {
			e.retry(q.tick)
		}
		sent++
		if sent == 32 {
			return
		}
	}
}

func (e *teamEvent) retry(tick uint64) {
	e.sending = false
	if e.delay == 0 {
		e.delay = 10 // SDK host ticks are 100 ms.
	} else {
		e.delay = min(2*e.delay, 600)
	}
	e.next = tick + e.delay
	log.Warnf("team report awaiting acknowledgement, requestId=%s, retryInTicks=%d", e.id, e.delay)
}
