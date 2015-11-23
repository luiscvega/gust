package gust

var deleteScript = `-- This script receives three parameters, all encoded with
-- MessagePack. The decoded values are used for deleting a model
-- instance in Redis and removing any reference to it in sets
-- (indices) and hashes (unique indices).
--
-- # model
--
-- Table with three attributes:
--    id (model instance id)
--    key (hash where the attributes will be saved)
--    name (model name)
--
-- # uniques
--
-- Fields and values to be removed from the unique indices.
--
-- # tracked
--
-- Keys that share the lifecycle of this model instance, that
-- should be removed as this object is deleted.
--
local model   = cjson.decode(ARGV[1])
model.key = model.name .. ":" .. model.id

local function remove_indices(model)
	local memo = model.key .. ":_indices"
	local existing = redis.call("SMEMBERS", memo)

	for _, key in ipairs(existing) do
		redis.call("SREM", key, model.id)
	end
end

local function remove_uniques(model)
	local memo = model.key .. ":_uniques"
	local existing = redis.call("HGETALL", memo)

	for i = 1, #existing, 2 do
		redis.call("HDEL", existing[i], existing[i+1])
	end
end

local function delete(model)
	local keys = {
		model.key .. ":_indices",
		model.key .. ":_uniques",
		model.key
	}

	redis.call("SREM", model.name .. ":all", model.id)
	return redis.call("DEL", unpack(keys))
end

remove_indices(model)
remove_uniques(model)
local count = delete(model)
return count>=1`
