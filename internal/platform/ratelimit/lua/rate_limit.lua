-- redis.call('del', KEYS[1]);
-- 参数说明:
-- KEYS[1]: 限流键
-- ARGV[1]: 请求数量
-- ARGV[2]: 生成令牌速率
-- ARGV[3]: 桶容量
local key = KEYS[1]
local now = redis.call('TIME')
local current = tonumber(now[1]) + tonumber(now[2]) / 1000000
local req = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])

local bucket = redis.call('HMGET', key, 'tokens', 'last_time')
local tokens = tonumber(bucket[1]) or capacity
local last_time = tonumber(bucket[2]) or current

tokens = math.min(capacity, tokens + (current - last_time) * rate)

if tokens < req then
    return 0
end

tokens = tokens - req
local ttl = math.ceil((capacity / rate) * 2)
redis.call('HMSET', key, 'tokens', tokens, 'last_time', current)
redis.call('EXPIRE', key, ttl)
return 1
