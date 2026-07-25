package stream

import gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"

type Result struct {
	status  *gatewaycontract.StreamStatus
	stopped bool
}

func newResult(status *gatewaycontract.StreamStatus) *Result {
	return &Result{status: status}
}

func (r *Result) Error(err error) {
	if err == nil {
		return
	}
	r.status.RecordError(err.Error())
}

func (r *Result) Stop(err error) {
	if err != nil {
		r.status.RecordError(err.Error())
	}
	r.status.SetEndReason(gatewaycontract.StreamEndReasonHandlerStop, err)
	r.stopped = true
}

func (r *Result) Done() {
	r.status.SetEndReason(gatewaycontract.StreamEndReasonDone, nil)
	r.stopped = true
}

func (r *Result) IsStopped() bool {
	return r.stopped
}

func (r *Result) reset() {
	r.stopped = false
}
