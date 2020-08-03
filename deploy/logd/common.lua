local logd = require("logd")

-- returns duration_us field in logptr, parsed as a
-- duration number in nanoseconds.
function parse_duration(logptr)
	local d = logd.log_get(logptr, "duration_us")
	local duration_us = tonumber(d)
	if duration_us == nil then
		return
	end

	return duration_us * 1000
end

return {
	latency_buckets = {
		5000, 100000, 500000, 1000000, 5000000, 10000000,
		50000000, 100000000, 500000000, 1000000000, 5000000000,
	}
}
