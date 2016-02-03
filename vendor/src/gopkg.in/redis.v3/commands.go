package redis

import (
	"io"
	"log"
	"strconv"
	"time"
)

func formatInt(i int64) string {
	return strconv.FormatInt(i, 10)
}

func formatUint(i uint64) string {
	return strconv.FormatUint(i, 10)
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func readTimeout(timeout time.Duration) time.Duration {
	if timeout == 0 {
		return 0
	}
	return timeout + time.Second
}

func usePrecise(dur time.Duration) bool {
	return dur < time.Second || dur%time.Second != 0
}

func formatMs(dur time.Duration) string {
	if dur > 0 && dur < time.Millisecond {
		log.Printf(
			"redis: specified duration is %s, but minimal supported value is %s",
			dur, time.Millisecond,
		)
	}
	return formatInt(int64(dur / time.Millisecond))
}

func formatSec(dur time.Duration) string {
	if dur > 0 && dur < time.Second {
		log.Printf(
			"redis: specified duration is %s, but minimal supported value is %s",
			dur, time.Second,
		)
	}
	return formatInt(int64(dur / time.Second))
}

type commandable struct {
	process func(cmd Cmder)
}

func (c *commandable) Process(cmd Cmder) {
	c.process(cmd)
}

//------------------------------------------------------------------------------

func (c *commandable) Auth(password string) *StatusCmd {
	cmd := newKeylessStatusCmd("AUTH", password)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Echo(message string) *StringCmd {
	cmd := NewStringCmd("ECHO", message)
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) Ping() *StatusCmd {
	cmd := newKeylessStatusCmd("PING")
	c.Process(cmd)
	return cmd
}

func (c *commandable) Quit() *StatusCmd {
	panic("not implemented")
}

func (c *commandable) Select(index int64) *StatusCmd {
	cmd := newKeylessStatusCmd("SELECT", index)
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) Del(keys ...string) *IntCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "DEL"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Dump(key string) *StringCmd {
	cmd := NewStringCmd("DUMP", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Exists(key string) *BoolCmd {
	cmd := NewBoolCmd("EXISTS", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Expire(key string, expiration time.Duration) *BoolCmd {
	cmd := NewBoolCmd("EXPIRE", key, formatSec(expiration))
	c.Process(cmd)
	return cmd
}

func (c *commandable) ExpireAt(key string, tm time.Time) *BoolCmd {
	cmd := NewBoolCmd("EXPIREAT", key, tm.Unix())
	c.Process(cmd)
	return cmd
}

func (c *commandable) Keys(pattern string) *StringSliceCmd {
	cmd := NewStringSliceCmd("KEYS", pattern)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Migrate(host, port, key string, db int64, timeout time.Duration) *StatusCmd {
	cmd := NewStatusCmd(
		"MIGRATE",
		host,
		port,
		key,
		db,
		formatMs(timeout),
	)
	cmd._clusterKeyPos = 3
	cmd.setReadTimeout(readTimeout(timeout))
	c.Process(cmd)
	return cmd
}

func (c *commandable) Move(key string, db int64) *BoolCmd {
	cmd := NewBoolCmd("MOVE", key, db)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ObjectRefCount(keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "OBJECT"
	args[1] = "REFCOUNT"
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(args...)
	cmd._clusterKeyPos = 2
	c.Process(cmd)
	return cmd
}

func (c *commandable) ObjectEncoding(keys ...string) *StringCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "OBJECT"
	args[1] = "ENCODING"
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewStringCmd(args...)
	cmd._clusterKeyPos = 2
	c.Process(cmd)
	return cmd
}

func (c *commandable) ObjectIdleTime(keys ...string) *DurationCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "OBJECT"
	args[1] = "IDLETIME"
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewDurationCmd(time.Second, args...)
	cmd._clusterKeyPos = 2
	c.Process(cmd)
	return cmd
}

func (c *commandable) Persist(key string) *BoolCmd {
	cmd := NewBoolCmd("PERSIST", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) PExpire(key string, expiration time.Duration) *BoolCmd {
	cmd := NewBoolCmd("PEXPIRE", key, formatMs(expiration))
	c.Process(cmd)
	return cmd
}

func (c *commandable) PExpireAt(key string, tm time.Time) *BoolCmd {
	cmd := NewBoolCmd(
		"PEXPIREAT",
		key,
		tm.UnixNano()/int64(time.Millisecond),
	)
	c.Process(cmd)
	return cmd
}

func (c *commandable) PTTL(key string) *DurationCmd {
	cmd := NewDurationCmd(time.Millisecond, "PTTL", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) RandomKey() *StringCmd {
	cmd := NewStringCmd("RANDOMKEY")
	c.Process(cmd)
	return cmd
}

func (c *commandable) Rename(key, newkey string) *StatusCmd {
	cmd := NewStatusCmd("RENAME", key, newkey)
	c.Process(cmd)
	return cmd
}

func (c *commandable) RenameNX(key, newkey string) *BoolCmd {
	cmd := NewBoolCmd("RENAMENX", key, newkey)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Restore(key string, ttl time.Duration, value string) *StatusCmd {
	cmd := NewStatusCmd(
		"RESTORE",
		key,
		formatMs(ttl),
		value,
	)
	c.Process(cmd)
	return cmd
}

func (c *commandable) RestoreReplace(key string, ttl time.Duration, value string) *StatusCmd {
	cmd := NewStatusCmd(
		"RESTORE",
		key,
		formatMs(ttl),
		value,
		"REPLACE",
	)
	c.Process(cmd)
	return cmd
}

type Sort struct {
	By            string
	Offset, Count float64
	Get           []string
	Order         string
	IsAlpha       bool
	Store         string
}

func (c *commandable) Sort(key string, sort Sort) *StringSliceCmd {
	args := []interface{}{"SORT", key}
	if sort.By != "" {
		args = append(args, "BY", sort.By)
	}
	if sort.Offset != 0 || sort.Count != 0 {
		args = append(args, "LIMIT", sort.Offset, sort.Count)
	}
	for _, get := range sort.Get {
		args = append(args, "GET", get)
	}
	if sort.Order != "" {
		args = append(args, sort.Order)
	}
	if sort.IsAlpha {
		args = append(args, "ALPHA")
	}
	if sort.Store != "" {
		args = append(args, "STORE", sort.Store)
	}
	cmd := NewStringSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) TTL(key string) *DurationCmd {
	cmd := NewDurationCmd(time.Second, "TTL", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Type(key string) *StatusCmd {
	cmd := NewStatusCmd("TYPE", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Scan(cursor int64, match string, count int64) *ScanCmd {
	args := []interface{}{"SCAN", cursor}
	if match != "" {
		args = append(args, "MATCH", match)
	}
	if count > 0 {
		args = append(args, "COUNT", count)
	}
	cmd := NewScanCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SScan(key string, cursor int64, match string, count int64) *ScanCmd {
	args := []interface{}{"SSCAN", key, cursor}
	if match != "" {
		args = append(args, "MATCH", match)
	}
	if count > 0 {
		args = append(args, "COUNT", count)
	}
	cmd := NewScanCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HScan(key string, cursor int64, match string, count int64) *ScanCmd {
	args := []interface{}{"HSCAN", key, cursor}
	if match != "" {
		args = append(args, "MATCH", match)
	}
	if count > 0 {
		args = append(args, "COUNT", count)
	}
	cmd := NewScanCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZScan(key string, cursor int64, match string, count int64) *ScanCmd {
	args := []interface{}{"ZSCAN", key, cursor}
	if match != "" {
		args = append(args, "MATCH", match)
	}
	if count > 0 {
		args = append(args, "COUNT", count)
	}
	cmd := NewScanCmd(args...)
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) Append(key, value string) *IntCmd {
	cmd := NewIntCmd("APPEND", key, value)
	c.Process(cmd)
	return cmd
}

type BitCount struct {
	Start, End int64
}

func (c *commandable) BitCount(key string, bitCount *BitCount) *IntCmd {
	args := []interface{}{"BITCOUNT", key}
	if bitCount != nil {
		args = append(
			args,
			bitCount.Start,
			bitCount.End,
		)
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) bitOp(op, destKey string, keys ...string) *IntCmd {
	args := make([]interface{}, 3+len(keys))
	args[0] = "BITOP"
	args[1] = op
	args[2] = destKey
	for i, key := range keys {
		args[3+i] = key
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) BitOpAnd(destKey string, keys ...string) *IntCmd {
	return c.bitOp("AND", destKey, keys...)
}

func (c *commandable) BitOpOr(destKey string, keys ...string) *IntCmd {
	return c.bitOp("OR", destKey, keys...)
}

func (c *commandable) BitOpXor(destKey string, keys ...string) *IntCmd {
	return c.bitOp("XOR", destKey, keys...)
}

func (c *commandable) BitOpNot(destKey string, key string) *IntCmd {
	return c.bitOp("NOT", destKey, key)
}

func (c *commandable) BitPos(key string, bit int64, pos ...int64) *IntCmd {
	args := make([]interface{}, 3+len(pos))
	args[0] = "BITPOS"
	args[1] = key
	args[2] = bit
	switch len(pos) {
	case 0:
	case 1:
		args[3] = pos[0]
	case 2:
		args[3] = pos[0]
		args[4] = pos[1]
	default:
		panic("too many arguments")
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Decr(key string) *IntCmd {
	cmd := NewIntCmd("DECR", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) DecrBy(key string, decrement int64) *IntCmd {
	cmd := NewIntCmd("DECRBY", key, decrement)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Get(key string) *StringCmd {
	cmd := NewStringCmd("GET", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) GetBit(key string, offset int64) *IntCmd {
	cmd := NewIntCmd("GETBIT", key, offset)
	c.Process(cmd)
	return cmd
}

func (c *commandable) GetRange(key string, start, end int64) *StringCmd {
	cmd := NewStringCmd("GETRANGE", key, start, end)
	c.Process(cmd)
	return cmd
}

func (c *commandable) GetSet(key string, value interface{}) *StringCmd {
	cmd := NewStringCmd("GETSET", key, value)
	c.Process(cmd)
	return cmd
}

func (c *commandable) Incr(key string) *IntCmd {
	cmd := NewIntCmd("INCR", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) IncrBy(key string, value int64) *IntCmd {
	cmd := NewIntCmd("INCRBY", key, value)
	c.Process(cmd)
	return cmd
}

func (c *commandable) IncrByFloat(key string, value float64) *FloatCmd {
	cmd := NewFloatCmd("INCRBYFLOAT", key, value)
	c.Process(cmd)
	return cmd
}

func (c *commandable) MGet(keys ...string) *SliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "MGET"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) MSet(pairs ...string) *StatusCmd {
	args := make([]interface{}, 1+len(pairs))
	args[0] = "MSET"
	for i, pair := range pairs {
		args[1+i] = pair
	}
	cmd := NewStatusCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) MSetNX(pairs ...string) *BoolCmd {
	args := make([]interface{}, 1+len(pairs))
	args[0] = "MSETNX"
	for i, pair := range pairs {
		args[1+i] = pair
	}
	cmd := NewBoolCmd(args...)
	c.Process(cmd)
	return cmd
}

// Redis `SET key value [expiration]` command.
//
// Zero expiration means the key has no expiration time.
func (c *commandable) Set(key string, value interface{}, expiration time.Duration) *StatusCmd {
	args := make([]interface{}, 3, 5)
	args[0] = "SET"
	args[1] = key
	args[2] = value
	if expiration > 0 {
		if usePrecise(expiration) {
			args = append(args, "PX", formatMs(expiration))
		} else {
			args = append(args, "EX", formatSec(expiration))
		}
	}
	cmd := NewStatusCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SetBit(key string, offset int64, value int) *IntCmd {
	cmd := NewIntCmd(
		"SETBIT",
		key,
		offset,
		value,
	)
	c.Process(cmd)
	return cmd
}

// Redis `SET key value [expiration] NX` command.
//
// Zero expiration means the key has no expiration time.
func (c *commandable) SetNX(key string, value interface{}, expiration time.Duration) *BoolCmd {
	var cmd *BoolCmd
	if expiration == 0 {
		// Use old `SETNX` to support old Redis versions.
		cmd = NewBoolCmd("SETNX", key, value)
	} else {
		if usePrecise(expiration) {
			cmd = NewBoolCmd("SET", key, value, "PX", formatMs(expiration), "NX")
		} else {
			cmd = NewBoolCmd("SET", key, value, "EX", formatSec(expiration), "NX")
		}
	}
	c.Process(cmd)
	return cmd
}

// Redis `SET key value [expiration] XX` command.
//
// Zero expiration means the key has no expiration time.
func (c *Client) SetXX(key string, value interface{}, expiration time.Duration) *BoolCmd {
	var cmd *BoolCmd
	if usePrecise(expiration) {
		cmd = NewBoolCmd("SET", key, value, "PX", formatMs(expiration), "XX")
	} else {
		cmd = NewBoolCmd("SET", key, value, "EX", formatSec(expiration), "XX")
	}
	c.Process(cmd)
	return cmd
}

func (c *commandable) SetRange(key string, offset int64, value string) *IntCmd {
	cmd := NewIntCmd("SETRANGE", key, offset, value)
	c.Process(cmd)
	return cmd
}

func (c *commandable) StrLen(key string) *IntCmd {
	cmd := NewIntCmd("STRLEN", key)
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) HDel(key string, fields ...string) *IntCmd {
	args := make([]interface{}, 2+len(fields))
	args[0] = "HDEL"
	args[1] = key
	for i, field := range fields {
		args[2+i] = field
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HExists(key, field string) *BoolCmd {
	cmd := NewBoolCmd("HEXISTS", key, field)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HGet(key, field string) *StringCmd {
	cmd := NewStringCmd("HGET", key, field)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HGetAll(key string) *StringSliceCmd {
	cmd := NewStringSliceCmd("HGETALL", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HGetAllMap(key string) *StringStringMapCmd {
	cmd := NewStringStringMapCmd("HGETALL", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HIncrBy(key, field string, incr int64) *IntCmd {
	cmd := NewIntCmd("HINCRBY", key, field, incr)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HIncrByFloat(key, field string, incr float64) *FloatCmd {
	cmd := NewFloatCmd("HINCRBYFLOAT", key, field, incr)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HKeys(key string) *StringSliceCmd {
	cmd := NewStringSliceCmd("HKEYS", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HLen(key string) *IntCmd {
	cmd := NewIntCmd("HLEN", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HMGet(key string, fields ...string) *SliceCmd {
	args := make([]interface{}, 2+len(fields))
	args[0] = "HMGET"
	args[1] = key
	for i, field := range fields {
		args[2+i] = field
	}
	cmd := NewSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HMSet(key, field, value string, pairs ...string) *StatusCmd {
	args := make([]interface{}, 4+len(pairs))
	args[0] = "HMSET"
	args[1] = key
	args[2] = field
	args[3] = value
	for i, pair := range pairs {
		args[4+i] = pair
	}
	cmd := NewStatusCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HSet(key, field, value string) *BoolCmd {
	cmd := NewBoolCmd("HSET", key, field, value)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HSetNX(key, field, value string) *BoolCmd {
	cmd := NewBoolCmd("HSETNX", key, field, value)
	c.Process(cmd)
	return cmd
}

func (c *commandable) HVals(key string) *StringSliceCmd {
	cmd := NewStringSliceCmd("HVALS", key)
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) BLPop(timeout time.Duration, keys ...string) *StringSliceCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "BLPOP"
	for i, key := range keys {
		args[1+i] = key
	}
	args[len(args)-1] = formatSec(timeout)
	cmd := NewStringSliceCmd(args...)
	cmd.setReadTimeout(readTimeout(timeout))
	c.Process(cmd)
	return cmd
}

func (c *commandable) BRPop(timeout time.Duration, keys ...string) *StringSliceCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "BRPOP"
	for i, key := range keys {
		args[1+i] = key
	}
	args[len(args)-1] = formatSec(timeout)
	cmd := NewStringSliceCmd(args...)
	cmd.setReadTimeout(readTimeout(timeout))
	c.Process(cmd)
	return cmd
}

func (c *commandable) BRPopLPush(source, destination string, timeout time.Duration) *StringCmd {
	cmd := NewStringCmd(
		"BRPOPLPUSH",
		source,
		destination,
		formatSec(timeout),
	)
	cmd.setReadTimeout(readTimeout(timeout))
	c.Process(cmd)
	return cmd
}

func (c *commandable) LIndex(key string, index int64) *StringCmd {
	cmd := NewStringCmd("LINDEX", key, index)
	c.Process(cmd)
	return cmd
}

func (c *commandable) LInsert(key, op, pivot, value string) *IntCmd {
	cmd := NewIntCmd("LINSERT", key, op, pivot, value)
	c.Process(cmd)
	return cmd
}

func (c *commandable) LLen(key string) *IntCmd {
	cmd := NewIntCmd("LLEN", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) LPop(key string) *StringCmd {
	cmd := NewStringCmd("LPOP", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) LPush(key string, values ...string) *IntCmd {
	args := make([]interface{}, 2+len(values))
	args[0] = "LPUSH"
	args[1] = key
	for i, value := range values {
		args[2+i] = value
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) LPushX(key, value interface{}) *IntCmd {
	cmd := NewIntCmd("LPUSHX", key, value)
	c.Process(cmd)
	return cmd
}

func (c *commandable) LRange(key string, start, stop int64) *StringSliceCmd {
	cmd := NewStringSliceCmd(
		"LRANGE",
		key,
		start,
		stop,
	)
	c.Process(cmd)
	return cmd
}

func (c *commandable) LRem(key string, count int64, value interface{}) *IntCmd {
	cmd := NewIntCmd("LREM", key, count, value)
	c.Process(cmd)
	return cmd
}

func (c *commandable) LSet(key string, index int64, value interface{}) *StatusCmd {
	cmd := NewStatusCmd("LSET", key, index, value)
	c.Process(cmd)
	return cmd
}

func (c *commandable) LTrim(key string, start, stop int64) *StatusCmd {
	cmd := NewStatusCmd(
		"LTRIM",
		key,
		start,
		stop,
	)
	c.Process(cmd)
	return cmd
}

func (c *commandable) RPop(key string) *StringCmd {
	cmd := NewStringCmd("RPOP", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) RPopLPush(source, destination string) *StringCmd {
	cmd := NewStringCmd("RPOPLPUSH", source, destination)
	c.Process(cmd)
	return cmd
}

func (c *commandable) RPush(key string, values ...string) *IntCmd {
	args := make([]interface{}, 2+len(values))
	args[0] = "RPUSH"
	args[1] = key
	for i, value := range values {
		args[2+i] = value
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) RPushX(key string, value interface{}) *IntCmd {
	cmd := NewIntCmd("RPUSHX", key, value)
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) SAdd(key string, members ...string) *IntCmd {
	args := make([]interface{}, 2+len(members))
	args[0] = "SADD"
	args[1] = key
	for i, member := range members {
		args[2+i] = member
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SCard(key string) *IntCmd {
	cmd := NewIntCmd("SCARD", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SDiff(keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "SDIFF"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStringSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SDiffStore(destination string, keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "SDIFFSTORE"
	args[1] = destination
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SInter(keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "SINTER"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStringSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SInterStore(destination string, keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "SINTERSTORE"
	args[1] = destination
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SIsMember(key string, member interface{}) *BoolCmd {
	cmd := NewBoolCmd("SISMEMBER", key, member)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SMembers(key string) *StringSliceCmd {
	cmd := NewStringSliceCmd("SMEMBERS", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SMove(source, destination string, member interface{}) *BoolCmd {
	cmd := NewBoolCmd("SMOVE", source, destination, member)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SPop(key string) *StringCmd {
	cmd := NewStringCmd("SPOP", key)
	c.Process(cmd)
	return cmd
}

// Redis `SRANDMEMBER key` command.
func (c *commandable) SRandMember(key string) *StringCmd {
	cmd := NewStringCmd("SRANDMEMBER", key)
	c.Process(cmd)
	return cmd
}

// Redis `SRANDMEMBER key count` command.
func (c *commandable) SRandMemberN(key string, count int64) *StringSliceCmd {
	cmd := NewStringSliceCmd("SRANDMEMBER", key, count)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SRem(key string, members ...string) *IntCmd {
	args := make([]interface{}, 2+len(members))
	args[0] = "SREM"
	args[1] = key
	for i, member := range members {
		args[2+i] = member
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SUnion(keys ...string) *StringSliceCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "SUNION"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewStringSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SUnionStore(destination string, keys ...string) *IntCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "SUNIONSTORE"
	args[1] = destination
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

// Z represents sorted set member.
type Z struct {
	Score  float64
	Member interface{}
}

// ZStore is used as an arg to ZInterStore and ZUnionStore.
type ZStore struct {
	Weights []float64
	// Can be SUM, MIN or MAX.
	Aggregate string
}

func (c *commandable) zAdd(a []interface{}, n int, members ...Z) *IntCmd {
	for i, m := range members {
		a[n+2*i] = m.Score
		a[n+2*i+1] = m.Member
	}
	cmd := NewIntCmd(a...)
	c.Process(cmd)
	return cmd
}

// Redis `ZADD key score member [score member ...]` command.
func (c *commandable) ZAdd(key string, members ...Z) *IntCmd {
	const n = 2
	a := make([]interface{}, n+2*len(members))
	a[0], a[1] = "ZADD", key
	return c.zAdd(a, n, members...)
}

// Redis `ZADD key NX score member [score member ...]` command.
func (c *commandable) ZAddNX(key string, members ...Z) *IntCmd {
	const n = 3
	a := make([]interface{}, n+2*len(members))
	a[0], a[1], a[2] = "ZADD", key, "NX"
	return c.zAdd(a, n, members...)
}

// Redis `ZADD key XX score member [score member ...]` command.
func (c *commandable) ZAddXX(key string, members ...Z) *IntCmd {
	const n = 3
	a := make([]interface{}, n+2*len(members))
	a[0], a[1], a[2] = "ZADD", key, "XX"
	return c.zAdd(a, n, members...)
}

// Redis `ZADD key CH score member [score member ...]` command.
func (c *commandable) ZAddCh(key string, members ...Z) *IntCmd {
	const n = 3
	a := make([]interface{}, n+2*len(members))
	a[0], a[1], a[2] = "ZADD", key, "CH"
	return c.zAdd(a, n, members...)
}

// Redis `ZADD key NX CH score member [score member ...]` command.
func (c *commandable) ZAddNXCh(key string, members ...Z) *IntCmd {
	const n = 4
	a := make([]interface{}, n+2*len(members))
	a[0], a[1], a[2], a[3] = "ZADD", key, "NX", "CH"
	return c.zAdd(a, n, members...)
}

// Redis `ZADD key XX CH score member [score member ...]` command.
func (c *commandable) ZAddXXCh(key string, members ...Z) *IntCmd {
	const n = 4
	a := make([]interface{}, n+2*len(members))
	a[0], a[1], a[2], a[3] = "ZADD", key, "XX", "CH"
	return c.zAdd(a, n, members...)
}

func (c *commandable) zIncr(a []interface{}, n int, members ...Z) *FloatCmd {
	for i, m := range members {
		a[n+2*i] = m.Score
		a[n+2*i+1] = m.Member
	}
	cmd := NewFloatCmd(a...)
	c.Process(cmd)
	return cmd
}

// Redis `ZADD key INCR score member` command.
func (c *commandable) ZIncr(key string, member Z) *FloatCmd {
	const n = 3
	a := make([]interface{}, n+2)
	a[0], a[1], a[2] = "ZADD", key, "INCR"
	return c.zIncr(a, n, member)
}

// Redis `ZADD key NX INCR score member` command.
func (c *commandable) ZIncrNX(key string, member Z) *FloatCmd {
	const n = 4
	a := make([]interface{}, n+2)
	a[0], a[1], a[2], a[3] = "ZADD", key, "INCR", "NX"
	return c.zIncr(a, n, member)
}

// Redis `ZADD key XX INCR score member` command.
func (c *commandable) ZIncrXX(key string, member Z) *FloatCmd {
	const n = 4
	a := make([]interface{}, n+2)
	a[0], a[1], a[2], a[3] = "ZADD", key, "INCR", "XX"
	return c.zIncr(a, n, member)
}

func (c *commandable) ZCard(key string) *IntCmd {
	cmd := NewIntCmd("ZCARD", key)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZCount(key, min, max string) *IntCmd {
	cmd := NewIntCmd("ZCOUNT", key, min, max)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZIncrBy(key string, increment float64, member string) *FloatCmd {
	cmd := NewFloatCmd("ZINCRBY", key, increment, member)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZInterStore(destination string, store ZStore, keys ...string) *IntCmd {
	args := make([]interface{}, 3+len(keys))
	args[0] = "ZINTERSTORE"
	args[1] = destination
	args[2] = strconv.Itoa(len(keys))
	for i, key := range keys {
		args[3+i] = key
	}
	if len(store.Weights) > 0 {
		args = append(args, "WEIGHTS")
		for _, weight := range store.Weights {
			args = append(args, weight)
		}
	}
	if store.Aggregate != "" {
		args = append(args, "AGGREGATE", store.Aggregate)
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) zRange(key string, start, stop int64, withScores bool) *StringSliceCmd {
	args := []interface{}{
		"ZRANGE",
		key,
		start,
		stop,
	}
	if withScores {
		args = append(args, "WITHSCORES")
	}
	cmd := NewStringSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZRange(key string, start, stop int64) *StringSliceCmd {
	return c.zRange(key, start, stop, false)
}

func (c *commandable) ZRangeWithScores(key string, start, stop int64) *ZSliceCmd {
	cmd := NewZSliceCmd("ZRANGE", key, start, stop, "WITHSCORES")
	c.Process(cmd)
	return cmd
}

// TODO: Rename to something more generic in v4
type ZRangeByScore struct {
	Min, Max      string
	Offset, Count int64
}

func (c *commandable) zRangeBy(zcmd, key string, opt ZRangeByScore, withScores bool) *StringSliceCmd {
	args := []interface{}{zcmd, key, opt.Min, opt.Max}
	if withScores {
		args = append(args, "WITHSCORES")
	}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"LIMIT",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewStringSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZRangeByScore(key string, opt ZRangeByScore) *StringSliceCmd {
	return c.zRangeBy("ZRANGEBYSCORE", key, opt, false)
}

func (c *commandable) ZRangeByLex(key string, opt ZRangeByScore) *StringSliceCmd {
	return c.zRangeBy("ZRANGEBYLEX", key, opt, false)
}

func (c *commandable) ZRangeByScoreWithScores(key string, opt ZRangeByScore) *ZSliceCmd {
	args := []interface{}{"ZRANGEBYSCORE", key, opt.Min, opt.Max, "WITHSCORES"}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"LIMIT",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewZSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZRank(key, member string) *IntCmd {
	cmd := NewIntCmd("ZRANK", key, member)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZRem(key string, members ...string) *IntCmd {
	args := make([]interface{}, 2+len(members))
	args[0] = "ZREM"
	args[1] = key
	for i, member := range members {
		args[2+i] = member
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZRemRangeByRank(key string, start, stop int64) *IntCmd {
	cmd := NewIntCmd(
		"ZREMRANGEBYRANK",
		key,
		start,
		stop,
	)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZRemRangeByScore(key, min, max string) *IntCmd {
	cmd := NewIntCmd("ZREMRANGEBYSCORE", key, min, max)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZRevRange(key string, start, stop int64) *StringSliceCmd {
	cmd := NewStringSliceCmd("ZREVRANGE", key, start, stop)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZRevRangeWithScores(key string, start, stop int64) *ZSliceCmd {
	cmd := NewZSliceCmd("ZREVRANGE", key, start, stop, "WITHSCORES")
	c.Process(cmd)
	return cmd
}

func (c *commandable) zRevRangeBy(zcmd, key string, opt ZRangeByScore) *StringSliceCmd {
	args := []interface{}{zcmd, key, opt.Max, opt.Min}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"LIMIT",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewStringSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZRevRangeByScore(key string, opt ZRangeByScore) *StringSliceCmd {
	return c.zRevRangeBy("ZREVRANGEBYSCORE", key, opt)
}

func (c commandable) ZRevRangeByLex(key string, opt ZRangeByScore) *StringSliceCmd {
	return c.zRevRangeBy("ZREVRANGEBYLEX", key, opt)
}

func (c *commandable) ZRevRangeByScoreWithScores(key string, opt ZRangeByScore) *ZSliceCmd {
	args := []interface{}{"ZREVRANGEBYSCORE", key, opt.Max, opt.Min, "WITHSCORES"}
	if opt.Offset != 0 || opt.Count != 0 {
		args = append(
			args,
			"LIMIT",
			opt.Offset,
			opt.Count,
		)
	}
	cmd := NewZSliceCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZRevRank(key, member string) *IntCmd {
	cmd := NewIntCmd("ZREVRANK", key, member)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZScore(key, member string) *FloatCmd {
	cmd := NewFloatCmd("ZSCORE", key, member)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ZUnionStore(dest string, store ZStore, keys ...string) *IntCmd {
	args := make([]interface{}, 3+len(keys))
	args[0] = "ZUNIONSTORE"
	args[1] = dest
	args[2] = strconv.Itoa(len(keys))
	for i, key := range keys {
		args[3+i] = key
	}
	if len(store.Weights) > 0 {
		args = append(args, "WEIGHTS")
		for _, weight := range store.Weights {
			args = append(args, weight)
		}
	}
	if store.Aggregate != "" {
		args = append(args, "AGGREGATE", store.Aggregate)
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) PFAdd(key string, fields ...string) *IntCmd {
	args := make([]interface{}, 2+len(fields))
	args[0] = "PFADD"
	args[1] = key
	for i, field := range fields {
		args[2+i] = field
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) PFCount(keys ...string) *IntCmd {
	args := make([]interface{}, 1+len(keys))
	args[0] = "PFCOUNT"
	for i, key := range keys {
		args[1+i] = key
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) PFMerge(dest string, keys ...string) *StatusCmd {
	args := make([]interface{}, 2+len(keys))
	args[0] = "PFMERGE"
	args[1] = dest
	for i, key := range keys {
		args[2+i] = key
	}
	cmd := NewStatusCmd(args...)
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) BgRewriteAOF() *StatusCmd {
	cmd := NewStatusCmd("BGREWRITEAOF")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) BgSave() *StatusCmd {
	cmd := NewStatusCmd("BGSAVE")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClientKill(ipPort string) *StatusCmd {
	cmd := NewStatusCmd("CLIENT", "KILL", ipPort)
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClientList() *StringCmd {
	cmd := NewStringCmd("CLIENT", "LIST")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClientPause(dur time.Duration) *BoolCmd {
	cmd := NewBoolCmd("CLIENT", "PAUSE", formatMs(dur))
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

// ClientSetName assigns a name to the one of many connections in the pool.
func (c *commandable) ClientSetName(name string) *BoolCmd {
	cmd := NewBoolCmd("CLIENT", "SETNAME", name)
	c.Process(cmd)
	return cmd
}

// ClientGetName returns the name of the one of many connections in the pool.
func (c *Client) ClientGetName() *StringCmd {
	cmd := NewStringCmd("CLIENT", "GETNAME")
	c.Process(cmd)
	return cmd
}

func (c *commandable) ConfigGet(parameter string) *SliceCmd {
	cmd := NewSliceCmd("CONFIG", "GET", parameter)
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ConfigResetStat() *StatusCmd {
	cmd := NewStatusCmd("CONFIG", "RESETSTAT")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ConfigSet(parameter, value string) *StatusCmd {
	cmd := NewStatusCmd("CONFIG", "SET", parameter, value)
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) DbSize() *IntCmd {
	cmd := NewIntCmd("DBSIZE")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) FlushAll() *StatusCmd {
	cmd := newKeylessStatusCmd("FLUSHALL")
	c.Process(cmd)
	return cmd
}

func (c *commandable) FlushDb() *StatusCmd {
	cmd := newKeylessStatusCmd("FLUSHDB")
	c.Process(cmd)
	return cmd
}

func (c *commandable) Info(section ...string) *StringCmd {
	args := []interface{}{"INFO"}
	if len(section) > 0 {
		args = append(args, section[0])
	}
	cmd := NewStringCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) LastSave() *IntCmd {
	cmd := NewIntCmd("LASTSAVE")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) Save() *StatusCmd {
	cmd := newKeylessStatusCmd("SAVE")
	c.Process(cmd)
	return cmd
}

func (c *commandable) shutdown(modifier string) *StatusCmd {
	var args []interface{}
	if modifier == "" {
		args = []interface{}{"SHUTDOWN"}
	} else {
		args = []interface{}{"SHUTDOWN", modifier}
	}
	cmd := newKeylessStatusCmd(args...)
	c.Process(cmd)
	if err := cmd.Err(); err != nil {
		if err == io.EOF {
			// Server quit as expected.
			cmd.err = nil
		}
	} else {
		// Server did not quit. String reply contains the reason.
		cmd.err = errorf(cmd.val)
		cmd.val = ""
	}
	return cmd
}

func (c *commandable) Shutdown() *StatusCmd {
	return c.shutdown("")
}

func (c *commandable) ShutdownSave() *StatusCmd {
	return c.shutdown("SAVE")
}

func (c *commandable) ShutdownNoSave() *StatusCmd {
	return c.shutdown("NOSAVE")
}

func (c *commandable) SlaveOf(host, port string) *StatusCmd {
	cmd := newKeylessStatusCmd("SLAVEOF", host, port)
	c.Process(cmd)
	return cmd
}

func (c *commandable) SlowLog() {
	panic("not implemented")
}

func (c *commandable) Sync() {
	panic("not implemented")
}

func (c *commandable) Time() *StringSliceCmd {
	cmd := NewStringSliceCmd("TIME")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) Eval(script string, keys []string, args []string) *Cmd {
	cmdArgs := make([]interface{}, 3+len(keys)+len(args))
	cmdArgs[0] = "EVAL"
	cmdArgs[1] = script
	cmdArgs[2] = strconv.Itoa(len(keys))
	for i, key := range keys {
		cmdArgs[3+i] = key
	}
	pos := 3 + len(keys)
	for i, arg := range args {
		cmdArgs[pos+i] = arg
	}
	cmd := NewCmd(cmdArgs...)
	if len(keys) > 0 {
		cmd._clusterKeyPos = 3
	}
	c.Process(cmd)
	return cmd
}

func (c *commandable) EvalSha(sha1 string, keys []string, args []string) *Cmd {
	cmdArgs := make([]interface{}, 3+len(keys)+len(args))
	cmdArgs[0] = "EVALSHA"
	cmdArgs[1] = sha1
	cmdArgs[2] = strconv.Itoa(len(keys))
	for i, key := range keys {
		cmdArgs[3+i] = key
	}
	pos := 3 + len(keys)
	for i, arg := range args {
		cmdArgs[pos+i] = arg
	}
	cmd := NewCmd(cmdArgs...)
	if len(keys) > 0 {
		cmd._clusterKeyPos = 3
	}
	c.Process(cmd)
	return cmd
}

func (c *commandable) ScriptExists(scripts ...string) *BoolSliceCmd {
	args := make([]interface{}, 2+len(scripts))
	args[0] = "SCRIPT"
	args[1] = "EXISTS"
	for i, script := range scripts {
		args[2+i] = script
	}
	cmd := NewBoolSliceCmd(args...)
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ScriptFlush() *StatusCmd {
	cmd := newKeylessStatusCmd("SCRIPT", "FLUSH")
	c.Process(cmd)
	return cmd
}

func (c *commandable) ScriptKill() *StatusCmd {
	cmd := newKeylessStatusCmd("SCRIPT", "KILL")
	c.Process(cmd)
	return cmd
}

func (c *commandable) ScriptLoad(script string) *StringCmd {
	cmd := NewStringCmd("SCRIPT", "LOAD", script)
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) DebugObject(key string) *StringCmd {
	cmd := NewStringCmd("DEBUG", "OBJECT", key)
	cmd._clusterKeyPos = 2
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) PubSubChannels(pattern string) *StringSliceCmd {
	args := []interface{}{"PUBSUB", "CHANNELS"}
	if pattern != "*" {
		args = append(args, pattern)
	}
	cmd := NewStringSliceCmd(args...)
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) PubSubNumSub(channels ...string) *StringIntMapCmd {
	args := make([]interface{}, 2+len(channels))
	args[0] = "PUBSUB"
	args[1] = "NUMSUB"
	for i, channel := range channels {
		args[2+i] = channel
	}
	cmd := NewStringIntMapCmd(args...)
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) PubSubNumPat() *IntCmd {
	cmd := NewIntCmd("PUBSUB", "NUMPAT")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

//------------------------------------------------------------------------------

func (c *commandable) ClusterSlots() *ClusterSlotCmd {
	cmd := NewClusterSlotCmd("CLUSTER", "slots")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterNodes() *StringCmd {
	cmd := NewStringCmd("CLUSTER", "nodes")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterMeet(host, port string) *StatusCmd {
	cmd := newKeylessStatusCmd("CLUSTER", "meet", host, port)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterForget(nodeID string) *StatusCmd {
	cmd := newKeylessStatusCmd("CLUSTER", "forget", nodeID)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterReplicate(nodeID string) *StatusCmd {
	cmd := newKeylessStatusCmd("CLUSTER", "replicate", nodeID)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterResetSoft() *StatusCmd {
	cmd := newKeylessStatusCmd("CLUSTER", "reset", "soft")
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterResetHard() *StatusCmd {
	cmd := newKeylessStatusCmd("CLUSTER", "reset", "hard")
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterInfo() *StringCmd {
	cmd := NewStringCmd("CLUSTER", "info")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterKeySlot(key string) *IntCmd {
	cmd := NewIntCmd("CLUSTER", "keyslot", key)
	cmd._clusterKeyPos = 2
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterCountFailureReports(nodeID string) *IntCmd {
	cmd := NewIntCmd("CLUSTER", "count-failure-reports", nodeID)
	cmd._clusterKeyPos = 2
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterCountKeysInSlot(slot int) *IntCmd {
	cmd := NewIntCmd("CLUSTER", "countkeysinslot", slot)
	cmd._clusterKeyPos = 2
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterDelSlots(slots ...int) *StatusCmd {
	args := make([]interface{}, 2+len(slots))
	args[0] = "CLUSTER"
	args[1] = "DELSLOTS"
	for i, slot := range slots {
		args[2+i] = slot
	}
	cmd := newKeylessStatusCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterDelSlotsRange(min, max int) *StatusCmd {
	size := max - min + 1
	slots := make([]int, size)
	for i := 0; i < size; i++ {
		slots[i] = min + i
	}
	return c.ClusterDelSlots(slots...)
}

func (c *commandable) ClusterSaveConfig() *StatusCmd {
	cmd := newKeylessStatusCmd("CLUSTER", "saveconfig")
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterSlaves(nodeID string) *StringSliceCmd {
	cmd := NewStringSliceCmd("CLUSTER", "SLAVES", nodeID)
	cmd._clusterKeyPos = 2
	c.Process(cmd)
	return cmd
}

func (c *commandable) Readonly() *StatusCmd {
	cmd := newKeylessStatusCmd("READONLY")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ReadWrite() *StatusCmd {
	cmd := newKeylessStatusCmd("READWRITE")
	cmd._clusterKeyPos = 0
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterFailover() *StatusCmd {
	cmd := newKeylessStatusCmd("CLUSTER", "failover")
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterAddSlots(slots ...int) *StatusCmd {
	args := make([]interface{}, 2+len(slots))
	args[0] = "CLUSTER"
	args[1] = "ADDSLOTS"
	for i, num := range slots {
		args[2+i] = strconv.Itoa(num)
	}
	cmd := newKeylessStatusCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) ClusterAddSlotsRange(min, max int) *StatusCmd {
	size := max - min + 1
	slots := make([]int, size)
	for i := 0; i < size; i++ {
		slots[i] = min + i
	}
	return c.ClusterAddSlots(slots...)
}

//------------------------------------------------------------------------------

func (c *commandable) GeoAdd(key string, geoLocation ...*GeoLocation) *IntCmd {
	args := make([]interface{}, 2+3*len(geoLocation))
	args[0] = "GEOADD"
	args[1] = key
	for i, eachLoc := range geoLocation {
		args[2+3*i] = eachLoc.Longitude
		args[2+3*i+1] = eachLoc.Latitude
		args[2+3*i+2] = eachLoc.Name
	}
	cmd := NewIntCmd(args...)
	c.Process(cmd)
	return cmd
}

func (c *commandable) GeoRadius(key string, longitude, latitude float64, query *GeoRadiusQuery) *GeoLocationCmd {
	cmd := NewGeoLocationCmd(query, "GEORADIUS", key, longitude, latitude)
	c.Process(cmd)
	return cmd
}

func (c *commandable) GeoRadiusByMember(key, member string, query *GeoRadiusQuery) *GeoLocationCmd {
	cmd := NewGeoLocationCmd(query, "GEORADIUSBYMEMBER", key, member)
	c.Process(cmd)
	return cmd
}

func (c *commandable) GeoDist(key string, member1, member2, unit string) *FloatCmd {
	if unit == "" {
		unit = "km"
	}
	cmd := NewFloatCmd("GEODIST", key, member1, member2, unit)
	c.Process(cmd)
	return cmd
}

func (c *commandable) GeoHash(key string, members ...string) *StringSliceCmd {
	args := make([]interface{}, 2+len(members))
	args[0] = "GEOHASH"
	args[1] = key
	for i, member := range members {
		args[2+i] = member
	}
	cmd := NewStringSliceCmd(args...)
	c.Process(cmd)
	return cmd
}
