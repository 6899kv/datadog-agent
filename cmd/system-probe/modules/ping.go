package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/DataDog/datadog-agent/cmd/system-probe/api/module"
	"github.com/DataDog/datadog-agent/cmd/system-probe/config"
	"github.com/DataDog/datadog-agent/cmd/system-probe/utils"
	pingcheck "github.com/DataDog/datadog-agent/pkg/networkdevice/pinger"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/gorilla/mux"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
)

const (
	countParam    = "count"
	intervalParam = "interval"
	timeoutParam  = "timeout"
)

type pinger struct{}

// Pinger is a factory for NDMs Ping module
var Pinger = module.Factory{
	Name:             config.PingModule,
	ConfigNamespaces: []string{"ping_config"},
	Fn: func(cfg *config.Config) (module.Module, error) {
		return &pinger{}, nil
	},
}

var _ module.Module = &pinger{}

func (p *pinger) GetStats() map[string]interface{} {
	return nil
}

func (p *pinger) Register(httpMux *module.Router) error {
	var runCounter = atomic.NewUint64(0)

	httpMux.HandleFunc("/ping/{host}", utils.WithConcurrencyLimit(utils.DefaultMaxConcurrentRequests, func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		vars := mux.Vars(req)
		id := getClientID(req)
		host := vars["host"]

		count, err := getIntParam(countParam, req)
		if err != nil {
			log.Errorf("unable to run ping invalid count %s: %s", host, err)
			w.Write([]byte(fmt.Sprintf("invalid count")))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		interval, err := getIntParam(intervalParam, req)
		if err != nil {
			log.Errorf("unable to run ping invalid interval %s: %s", host, err)
			w.Write([]byte(fmt.Sprintf("invalid interval")))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		timeout, err := getIntParam(timeoutParam, req)
		if err != nil {
			log.Errorf("unable to run ping invalid timeout %s: %s", host, err)
			w.Write([]byte(fmt.Sprintf("invalid timeout")))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		cfg := pingcheck.Config{
			UseRawSocket: true,
			Interval:     time.Duration(interval),
			Timeout:      time.Duration(timeout),
			Count:        count,
		}

		// Run ping using raw socket
		result, err := pingcheck.RunPing(&cfg, host)
		if err != nil {
			log.Errorf("unable to run ping for host %s: %s", host, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp, err := json.Marshal(result)
		if err != nil {
			log.Errorf("unable to marshall ping stats: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, err = w.Write(resp)
		if err != nil {
			log.Errorf("unable to write ping response: %s", err)
		}

		runCount := runCounter.Inc()
		logPingRequests(host, id, count, interval, timeout, runCount, start)
	}))

	return nil
}

func (p *pinger) RegisterGRPC(_ grpc.ServiceRegistrar) error {
	return nil
}

func (p *pinger) Close() {}

func logPingRequests(host string, client string, count int, interval int, timeout int, runCount uint64, start time.Time) {
	args := []interface{}{host, client, count, interval, timeout, runCount, time.Since(start)}
	msg := "Got request on /ping/%s?client_id=%s&count=%d&interval=%d&timeout=%d (count: %d): retrieved ping in %s"
	switch {
	case count <= 5, count%20 == 0:
		log.Infof(msg, args...)
	default:
		log.Debugf(msg, args...)
	}
}

func getIntParam(name string, req *http.Request) (int, error) {
	// only return an error if the param is present
	if req.URL.Query().Has(name) {
		return strconv.Atoi(req.URL.Query().Get(name))
	}

	return 0, nil
}
