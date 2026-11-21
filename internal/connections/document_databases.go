package connections

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type redisDatabase struct {
	client  *redis.Client
	options redis.Options
	in      Input
}

func openRedis(ctx context.Context, in Input, password string) (Database, error) {
	index, err := redisIndex(in.Database)
	if err != nil {
		return nil, err
	}
	t, err := connectionTLS(in)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(in.Timeout) * time.Second
	opts := redis.Options{Addr: net.JoinHostPort(in.Host, strconv.Itoa(in.Port)), Username: in.Username, Password: password, DB: index, TLSConfig: t, DialTimeout: timeout, ReadTimeout: timeout, WriteTimeout: timeout, ContextTimeoutEnabled: true, MaxRetries: -1, PoolSize: 4}
	client := redis.NewClient(&opts)
	if err = client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, connectionError("redis", err)
	}
	return &redisDatabase{client: client, options: opts, in: in}, nil
}
func (d *redisDatabase) Close() error { return d.client.Close() }
func (d *redisDatabase) Info(ctx context.Context) (SessionInfo, error) {
	info := SessionInfo{Connected: true, ServerVersion: "Redis", CurrentUser: d.in.Username, Databases: []string{}}
	if err := d.client.Ping(ctx).Err(); err != nil {
		return info, connectionError("redis", err)
	}
	if text, err := d.client.Info(ctx, "server").Result(); err == nil {
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "redis_version:") {
				info.ServerVersion = "Redis " + strings.TrimSpace(strings.TrimPrefix(line, "redis_version:"))
			}
		}
	}
	names := map[int]bool{d.options.DB: true}
	if text, err := d.client.Info(ctx, "keyspace").Result(); err == nil {
		for _, line := range strings.Split(text, "\n") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && strings.HasPrefix(parts[0], "db") {
				if n, e := redisIndex(strings.TrimPrefix(parts[0], "db")); e == nil {
					names[n] = true
				}
			}
		}
	}
	var indexes []int
	for n := range names {
		indexes = append(indexes, n)
	}
	sort.Ints(indexes)
	for _, n := range indexes {
		info.Databases = append(info.Databases, strconv.Itoa(n))
	}
	if info.CurrentUser == "" {
		info.CurrentUser = "default"
	}
	return info, nil
}
func (d *redisDatabase) forDB(database string) (*redis.Client, error) {
	n, err := redisIndex(database)
	if err != nil {
		return nil, err
	}
	opts := d.options
	opts.DB = n
	return redis.NewClient(&opts), nil
}
func redisKeyArgument(key string) any {
	if utf8.ValidString(key) {
		return key
	}
	return map[string]string{"hex": hex.EncodeToString([]byte(key))}
}
func (d *redisDatabase) Objects(ctx context.Context, database, kind string) (ObjectList, error) {
	list := ObjectList{Objects: []DatabaseObject{}}
	if kind != "tables" {
		return list, nil
	}
	client, err := d.forDB(database)
	if err != nil {
		return list, err
	}
	defer client.Close()
	var cursor uint64
	seen := map[string]bool{}
	for {
		keys, next, e := client.Scan(ctx, cursor, "*", 500).Result()
		if e != nil {
			return list, e
		}
		pipeline := client.Pipeline()
		typeCommands := make([]*redis.StatusCmd, len(keys))
		for i, key := range keys {
			typeCommands[i] = pipeline.Type(ctx, key)
		}
		if _, e = pipeline.Exec(ctx); e != nil {
			return list, e
		}
		for keyIndex, key := range keys {
			if seen[key] {
				continue
			}
			seen[key] = true
			if len(list.Objects) >= 10000 {
				list.Truncated = true
				return list, nil
			}
			typ, e := typeCommands[keyIndex].Result()
			if e != nil {
				return list, e
			}
			args := []any{"GET", redisKeyArgument(key)}
			switch typ {
			case "list":
				args = []any{"LRANGE", redisKeyArgument(key), "0", "999"}
			case "hash":
				args = []any{"HSCAN", redisKeyArgument(key), "0", "COUNT", "1000"}
			case "set":
				args = []any{"SSCAN", redisKeyArgument(key), "0", "COUNT", "1000"}
			case "zset":
				args = []any{"ZRANGE", redisKeyArgument(key), "0", "999", "WITHSCORES"}
			case "stream":
				args = []any{"XRANGE", redisKeyArgument(key), "-", "+", "COUNT", "1000"}
			case "string":
			default:
				args = []any{"TYPE", redisKeyArgument(key)}
			}
			command, _ := json.Marshal(args)
			// Quoting every key is injective, including empty/binary keys and
			// literal strings that resemble a HEX display marker.
			name := strconv.Quote(key)
			list.Objects = append(list.Objects, DatabaseObject{Name: name, Kind: "tables", Engine: typ, Query: string(command)})
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	sort.Slice(list.Objects, func(i, j int) bool { return list.Objects[i].Name < list.Objects[j].Name })
	return list, nil
}
func (d *redisDatabase) Admin(ctx context.Context, database string, request AdminRequest) (AdminResult, error) {
	result := AdminResult{Results: []QueryResult{}}
	if request.Operation != "query" {
		return result, errors.New("Redis 请使用键浏览和 JSON 数组原生命令")
	}
	if len(request.SQL) > 8<<20 {
		return result, errors.New("命令超过 8 MB")
	}
	var raw []json.RawMessage
	if json.Unmarshal([]byte(request.SQL), &raw) != nil || len(raw) == 0 {
		return result, errors.New("请输入 Redis 命令 JSON 数组，例如 [\"GET\",\"key\"]")
	}
	args := make([]any, len(raw))
	for i, value := range raw {
		var text string
		if json.Unmarshal(value, &text) == nil {
			args[i] = text
			continue
		}
		var encoded map[string]string
		if json.Unmarshal(value, &encoded) == nil && len(encoded) == 1 {
			if h, ok := encoded["hex"]; ok {
				b, e := hex.DecodeString(h)
				if e != nil {
					return result, errors.New("HEX 参数无效")
				}
				args[i] = string(b)
				continue
			}
		}
		dec := json.NewDecoder(bytes.NewReader(value))
		dec.UseNumber()
		var scalar any
		if dec.Decode(&scalar) != nil {
			return result, errors.New("命令参数无效")
		}
		switch v := scalar.(type) {
		case json.Number:
			args[i] = string(v)
		case bool:
			args[i] = strconv.FormatBool(v)
		default:
			return result, errors.New("命令参数须为字符串、数字或 HEX 对象")
		}
	}
	command, ok := args[0].(string)
	if !ok || command == "" {
		return result, errors.New("命令名称无效")
	}
	for _, blocked := range []string{"MONITOR", "SUBSCRIBE", "PSUBSCRIBE", "SSUBSCRIBE"} {
		if strings.EqualFold(command, blocked) {
			return result, errors.New("查询窗口不支持持续订阅或 MONITOR，请使用外部 Redis CLI")
		}
	}
	client, err := d.forDB(database)
	if err != nil {
		return result, err
	}
	defer client.Close()
	value, err := client.Do(ctx, args...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return result, err
	}
	q := QueryResult{Columns: []string{"结果"}, Rows: [][]*string{{nil}}}
	if err == nil {
		var text string
		if v, ok := value.(string); ok {
			if utf8.ValidString(v) {
				text = v
			} else {
				text = "0x" + hex.EncodeToString([]byte(v))
			}
		} else {
			encoded, e := json.Marshal(value)
			if e != nil {
				text = fmt.Sprint(value)
			} else {
				text = string(encoded)
			}
		}
		if len(text) > 8<<20 {
			return result, errors.New("命令结果超过 8 MB，请分批查询")
		}
		q.Rows = [][]*string{{&text}}
	}
	result.Results = append(result.Results, q)
	result.Completed = 1
	return result, nil
}

type mongoDatabase struct {
	client *mongo.Client
	in     Input
}

func openMongo(ctx context.Context, in Input, password string) (Database, error) {
	t, err := connectionTLS(in)
	if err != nil {
		return nil, err
	}
	if t != nil && in.ServerName == "" {
		t.ServerName = ""
	} // Verify each discovered replica-set endpoint by its own hostname.
	timeout := time.Duration(in.Timeout) * time.Second
	opts := options.Client().SetHosts([]string{net.JoinHostPort(in.Host, strconv.Itoa(in.Port))}).SetConnectTimeout(timeout).SetServerSelectionTimeout(timeout).SetTimeout(timeout).SetMaxPoolSize(4).SetAppName("BajiuDB").SetTLSConfig(t)
	if in.Username != "" {
		opts.SetAuth(options.Credential{Username: in.Username, Password: password, PasswordSet: true, AuthSource: in.AuthDatabase})
	} else if password != "" {
		return nil, errors.New("MongoDB 密码认证需要填写用户名")
	}
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, connectionError("mongodb", err)
	}
	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		client.Disconnect(closeCtx)
		return nil, connectionError("mongodb", err)
	}
	return &mongoDatabase{client: client, in: in}, nil
}
func (d *mongoDatabase) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return d.client.Disconnect(ctx)
}
func (d *mongoDatabase) Info(ctx context.Context) (SessionInfo, error) {
	info := SessionInfo{Connected: true, ServerVersion: "MongoDB", CurrentUser: d.in.Username, Databases: []string{}}
	if err := d.client.Ping(ctx, readpref.Primary()); err != nil {
		return info, connectionError("mongodb", err)
	}
	var build struct {
		Version string `bson:"version"`
	}
	if d.client.Database("admin").RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&build) == nil {
		info.ServerVersion = "MongoDB " + build.Version
	}
	names, err := d.client.ListDatabaseNames(ctx, bson.D{}, options.ListDatabases().SetAuthorizedDatabases(true))
	if err != nil {
		if d.in.Database == "" {
			return info, errors.New("无法列出 MongoDB 数据库，请填写账户可访问的默认数据库")
		}
		if e := d.client.Database(d.in.Database).RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err(); e != nil {
			return info, e
		}
		names = []string{d.in.Database}
	}
	if len(names) > 10000 {
		return info, errors.New("数据库超过 10,000 个")
	}
	sort.Strings(names)
	info.Databases = names
	return info, nil
}
func (d *mongoDatabase) Objects(ctx context.Context, database, kind string) (ObjectList, error) {
	list := ObjectList{Objects: []DatabaseObject{}}
	if kind != "tables" && kind != "views" {
		return list, nil
	}
	collectionType := "collection"
	if kind == "views" {
		collectionType = "view"
	}
	cursor, err := d.client.Database(database).ListCollections(ctx, bson.D{{Key: "type", Value: collectionType}}, options.ListCollections().SetAuthorizedCollections(true).SetNameOnly(true))
	if err != nil {
		return list, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		if len(list.Objects) >= 10000 {
			list.Truncated = true
			break
		}
		var spec struct {
			Name string `bson:"name"`
		}
		if err = cursor.Decode(&spec); err != nil {
			return list, err
		}
		query := mongoPreviewCommand(spec.Name)
		list.Objects = append(list.Objects, DatabaseObject{Name: spec.Name, Kind: kind, Engine: "MongoDB", Query: string(query)})
	}
	sort.Slice(list.Objects, func(i, j int) bool { return list.Objects[i].Name < list.Objects[j].Name })
	return list, cursor.Err()
}
func mongoPreviewCommand(name string) []byte {
	query, _ := bson.MarshalExtJSON(bson.D{{Key: "find", Value: name}, {Key: "filter", Value: bson.D{}}, {Key: "limit", Value: 1000}, {Key: "singleBatch", Value: true}}, false, false)
	return query
}
func (d *mongoDatabase) Admin(ctx context.Context, database string, request AdminRequest) (AdminResult, error) {
	result := AdminResult{Results: []QueryResult{}}
	if request.Operation != "query" {
		return result, errors.New("MongoDB 请使用集合浏览或原生 JSON 命令")
	}
	if len(request.SQL) > 8<<20 {
		return result, errors.New("命令超过 8 MB")
	}
	var command bson.D
	if bson.UnmarshalExtJSON([]byte(request.SQL), false, &command) != nil || len(command) == 0 {
		return result, errors.New("请输入 MongoDB JSON 命令，例如 {\"find\":\"users\",\"filter\":{},\"limit\":1000}")
	}
	raw, err := d.client.Database(database).RunCommand(ctx, command).Raw()
	if err != nil {
		return result, err
	}
	encoded, err := bson.MarshalExtJSON(raw, true, false)
	if err != nil {
		return result, err
	}
	if len(encoded) > 8<<20 {
		return result, errors.New("命令响应超过 8 MB，请限制结果范围")
	}
	value := string(encoded)
	result.Results = []QueryResult{{Columns: []string{"Extended JSON 结果"}, Rows: [][]*string{{&value}}}}
	result.Completed = 1
	return result, nil
}
