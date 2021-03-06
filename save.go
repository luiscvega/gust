package gust

var saveScript = `local model   = cjson.decode(ARGV[1])
local attrs   = ARGV[2]
local indices = cjson.decode(ARGV[3])
local uniques = cjson.decode(ARGV[4])

local function save(model, attrs)
	model.key = model.name .. ":" .. model.id

	redis.call("SADD", model.name .. ":all", model.id)
	redis.call("DEL", model.key)
	redis.call("SET", model.key, attrs)
end

local function index(model, indices)
	for field, value in pairs(indices) do
		local key = model.name .. ":indices:" .. field .. ":" .. tostring(value)

		redis.call("SADD", model.key .. ":_indices", key)
		redis.call("SADD", key, model.id)
	end
end

local function remove_indices(model)
	local memo = model.key .. ":_indices"
	local existing = redis.call("SMEMBERS", memo)

	for _, key in ipairs(existing) do
		redis.call("SREM", key, model.id)
		redis.call("SREM", memo, key)
	end
end

local function unique(model, uniques)
	for field, value in pairs(uniques) do
		local key = model.name .. ":uniques:" .. field

		redis.call("HSET", model.key .. ":_uniques", key, value)
		redis.call("HSET", key, value, model.id)
	end
end

local function remove_uniques(model)
	local memo = model.key .. ":_uniques"

	for _, key in pairs(redis.call("HKEYS", memo)) do
		redis.call("HDEL", key, redis.call("HGET", memo, key))
		redis.call("HDEL", memo, key)
	end
end

local function verify(model, uniques)
	local duplicates = {}

	for field, value in pairs(uniques) do
		local key = model.name .. ":uniques:" .. field
		local id = redis.call("HGET", key, tostring(value))

		if id and id ~= tostring(model.id) then
			duplicates[#duplicates + 1] = field
		end
	end

	return duplicates, #duplicates ~= 0
end

local duplicates, err = verify(model, uniques)

if err then
	error("UniqueIndexViolation: " .. duplicates[1])
end

save(model, attrs)

remove_indices(model)
index(model, indices)

remove_uniques(model, uniques)
unique(model, uniques)`
