--
-- This logd script is designed to consume logs from gps-producer
-- Before running, make sure you install all the dependencies via `lit install`.
--
local logd = require('logd')
local http = require('http')
local common = require('common')
local prometheus_prefix = 'gps_producer_'

-- init prometheus module
local prometheus = common.init_prometheus(require('prometheus'), prometheus_prefix)

local logs = prometheus:counter('logs', 'Total logs processed counter', {'status'})
local errors = prometheus:counter('errors', 'Total errors either as ERROR level or failure step', {'class', 'callType'})
local position_data = prometheus:gauge('gps_data',
	'GPS position by "metric" (Altitude, Latitude or Longitude)', {'metric'})
local position_lat = prometheus:histogram('position_latency_ns',
	'Latency of successful gps.PositionerPosition in nanoseconds', nil,
	common.latency_buckets)

local function record_position(logptr)
	local alt_raw = logd.log_get(logptr, 'Altitude')
	if alt_raw ~= nil then
		position_data:set(tonumber(alt_raw), {'Altitude'})
	end

	local lat_raw = logd.log_get(logptr, 'Latitude')
	if lat_raw ~= nil then
		position_data:set(tonumber(lat_raw), {'Latitude'})
	end

	local lng_raw = logd.log_get(logptr, 'Longitude')
	if lng_raw ~= nil then
		position_data:set(tonumber(lng_raw), {'Longitude'})
	end
end

local function record_position_lat(logptr, duration)
	position_lat:observe(duration)
end

function logd.on_log(logptr)
	logs:inc(1, {'success'})
		
	local k = logd.log_get(logptr, 'class')
	local ct = logd.log_get(logptr, 'callType')
	local l = logd.log_get(logptr, 'level')
	local s = logd.log_get(logptr, 'step')

	if l == 'ERROR' or s == 'failure' then
		errors:inc(1, {k, ct})
		return
	end

	if s ~= 'success' then
		return
	end

	local duration = common.parse_duration(logptr)
	if duration == nil then
		return
	end

	if ct == 'gps.PositionerPosition' then
		record_position(logptr)
		record_position_lat(logptr, duration)
	end
end

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

setup_metrics_server(8080)
