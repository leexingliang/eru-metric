package falcon

import (
	"math"
	"net/rpc"
	"sync"
	"time"

	"github.com/HunanTV/eru-agent/logs"
	"github.com/open-falcon/common/model"
	"github.com/toolkits/net"
)

type SingleConnRpcClient struct {
	sync.Mutex
	rpcClient *rpc.Client
	RpcServer string
	Timeout   time.Duration
}

func (self *SingleConnRpcClient) Close() {
	if self.rpcClient != nil {
		self.rpcClient.Close()
		self.rpcClient = nil
	}
}

func (self *SingleConnRpcClient) insureConn() error {
	if self.rpcClient != nil {
		return nil
	}

	var err error
	var retry int = 1

	for {
		if self.rpcClient != nil {
			return nil
		}

		self.rpcClient, err = net.JsonRpcClient("tcp", self.RpcServer, self.Timeout)
		if err == nil {
			return nil
		}

		logs.Info("Metrics rpc dial fail", err)
		if retry > 5 {
			return err
		}

		time.Sleep(time.Duration(math.Pow(2.0, float64(retry))) * time.Second)
		retry++
	}
	return nil
}

func (self *SingleConnRpcClient) Call(method string, args interface{}, reply interface{}) error {
	self.Lock()
	defer self.Unlock()

	if err := self.insureConn(); err != nil {
		return err
	}

	timeout := time.Duration(50 * time.Second)
	done := make(chan error)

	go func() {
		err := self.rpcClient.Call(method, args, reply)
		done <- err
	}()

	select {
	case <-time.After(timeout):
		logs.Info("Metrics rpc call timeout", self.rpcClient, self.RpcServer)
		self.Close()
	case err := <-done:
		if err != nil {
			self.Close()
			return err
		}
	}

	return nil
}

func (self SingleConnRpcClient) Send(data map[string]float64, endpoint, tag string, timestamp, step int64) error {
	metrics := []*model.MetricValue{}
	var metric *model.MetricValue
	for k, d := range data {
		metric = self.newMetricValue(k, d, endpoint, tag, timestamp, step)
		metrics = append(metrics, metric)
	}
	var resp model.TransferResponse
	if err := self.Call("Transfer.Update", metrics, &resp); err != nil {
		return err
	}
	logs.Debug(data)
	logs.Debug(endpoint, timestamp, &resp)
	return nil
}

func (self SingleConnRpcClient) newMetricValue(metric string, value interface{}, endpoint, tag string, timestamp, step int64) *model.MetricValue {
	mv := &model.MetricValue{
		Endpoint:  endpoint,
		Metric:    metric,
		Value:     value,
		Step:      step,
		Type:      "GAUGE",
		Tags:      tag,
		Timestamp: timestamp,
	}
	return mv
}