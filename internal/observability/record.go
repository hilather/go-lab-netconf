package observability

// ObserveRPC increments labnetconf_rpcs_total. A nil registry is a no-op.
func ObserveRPC(r *Registry, rpc, decision string) {
	if r == nil {
		return
	}
	r.Inc(MetricRPCsTotal, map[string]string{
		"rpc":      RPCName(rpc),
		"decision": decisionOrError(decision),
	}, 1)
}

func decisionOrError(d string) string {
	switch d {
	case "ok", "error":
		return d
	default:
		return RPCDecision(false)
	}
}

// ObserveRESTCONF increments labnetconf_restconf_requests_total.
func ObserveRESTCONF(r *Registry, method string, status int) {
	if r == nil {
		return
	}
	r.Inc(MetricRESTCONFRequestsTotal, map[string]string{
		"method": RESTCONFMethod(method),
		"code":   HTTPCode(status),
	}, 1)
}

// ObserveHTTP increments labnetconf_http_requests_total.
func ObserveHTTP(r *Registry, status int, route string) {
	if r == nil {
		return
	}
	r.Inc(MetricHTTPRequestsTotal, map[string]string{
		"code":  HTTPCode(status),
		"route": HTTPRoute(route),
	}, 1)
}

// ObserveApply increments labnetconf_apply_total.
func ObserveApply(r *Registry, result string) {
	if r == nil {
		return
	}
	r.Inc(MetricApplyTotal, map[string]string{"result": ApplyResult(result)}, 1)
}

// SetSessions writes labnetconf_sessions.
func SetSessions(r *Registry, n int) {
	if r == nil {
		return
	}
	r.Set(MetricSessions, nil, float64(n))
}

// SetLocks writes labnetconf_locks.
func SetLocks(r *Registry, n int) {
	if r == nil {
		return
	}
	r.Set(MetricLocks, nil, float64(n))
}

// SetNotifications writes labnetconf_notifications.
func SetNotifications(r *Registry, n int) {
	if r == nil {
		return
	}
	r.Set(MetricNotifications, nil, float64(n))
}
