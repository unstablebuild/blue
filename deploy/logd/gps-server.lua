--
-- This logd script is designed to consume logs from gps-server
-- Before running, make sure you install all the dependencies via `lit install`.
--
local logd = require("logd")
local http = require('http')
local common = require('common')
local prometheus_prefix = 'gps_server_'

-- init prometheus module
local prometheus = require('prometheus').init(prometheus_prefix, {
	log = function(level, msg)
		logd.print({
			level = level,
			msg = "Prometheus error: " .. msg
		})
	end
})

local logs = prometheus:counter('logs', 'Total logs processed counter', {'status'})
local errors = prometheus:counter('errors', 'Total errors either as ERROR level or failure step', {'class', "callType"})
local rec_lat = prometheus:histogram('receives_latency_ns',
	'Latency of successful Receivers.Receive in nanoseconds',
	{'class', "callType"},
	common.latency_buckets)
local http_lat = prometheus:histogram('http_latency_ns',
	'HTTP serving latency',
	{"status", "uri"},
	common.latency_buckets)

-- collect metrics and satisfy request
local function on_metrics_request(req, res)
	local body = prometheus:collect()
	res:setHeader("Content-Type", "text/plain")
	res:setHeader("Content-Length", #body)
	res:finish(body)
end

local function setup_metrics_server(port)
	http.createServer(on_metrics_request):listen(port)
	logd.print("Prometheus exporter listening at http://localhost:8080/")
end

local function record_receive(logptr, duration)
	local k = logd.log_get(logptr, "class")
	local ct = logd.log_get(logptr, "callType")
	rec_lat:observe(duration, {k, ct})
end

local function record_serve_http(logptr, duration)
	local status = logd.log_get(logptr, "status")
	local uri = logd.log_get(logptr, "uri")
	if status == nil or uri == nil then
		logd.log_set(logptr, "WARNING", "status or uri is nil on ServeHTTP log")
		logd.print(logptr)
		return
	end
	http_lat:observe(duration, {status, uri})
end

function logd.on_log(logptr)
	logs:inc(1, {'success'})
		
	local k = logd.log_get(logptr, "class")
	local ct = logd.log_get(logptr, "callType")
	local l = logd.log_get(logptr, "level")
	local s = logd.log_get(logptr, "step")

	if l == "ERROR" or s == "failure" then
		errors:inc(1, {k, ct})
		return
	end

	if s ~= "success" then
		return
	end

	local duration = parse_duration(logptr)
	if duration == nil then
		return
	end

	if ct == "Receive" then
		record_receive(logptr, duration)
	elseif ct == "ServeHTTP" then
		record_serve_http(logptr, duration)
	end
end

function logd.on_error(msg, logptr, at)
	logs:inc(1, {'failure'})

	logd.print({
		level = 'ERROR',
		err = msg,
		at = at,
		partial = logd.to_str(logptr),
	})
end

setup_metrics_server(8080)
