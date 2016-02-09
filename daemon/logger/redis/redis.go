// +build linux

// Package redis provides the log driver for forwarding server logs to redis
package redis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/docker/pkg/urlutil"
	"gopkg.in/redis.v3"
)

const name = "redis"

type redisLogger struct {
	client    *redis.Client
	ctx       logger.Context
	key       string
	container string
	tag       string
	hostname  string
	extra     map[string]string
}

type message struct {
	Message   string            `json:"message"`
	Container string            `json:"container"`
	Host      string            `json:"host"`
	Tag       string            `json:"tag"`
	Attrs     map[string]string `json:"attrs"`
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates a redis logger using the configuration passed in on the
// context. Supported context configuration variables are
// redis-address, & redis-key.
func New(ctx logger.Context) (logger.Logger, error) {
	address, err := parseAddress(ctx.Config["redis-address"])
	if err != nil {
		return nil, err
	}

	hostname, err := ctx.Hostname()
	if err != nil {
		return nil, fmt.Errorf("redis: cannot access hostname to set source field")
	}

	containerName := bytes.TrimLeft([]byte(ctx.ContainerName), "/")

	// parse log tag
	fmt.Printf("%+v\n", ctx)
	tag, err := loggerutils.ParseLogTag(ctx, "")
	if err != nil {
		return nil, err
	}

	//change database index if provided
	var redisDatabase int64
	if len(ctx.Config["redis-database"]) > 0 {
		redisDatabase, err = strconv.ParseInt(ctx.Config["redis-database"], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("redis: cannot parse database index: %s %v", ctx.Config["redis-database"], err)
		}
	}

	// create new redisClient
	redisClient := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: ctx.Config["redis-password"],
		DB:       redisDatabase,
	})

	var redisKey string
	if redisKey = ctx.Config["redis-key"]; len(ctx.Config["redis-key"]) == 0 {
		redisKey = "docker-logger"
	}

	if err != nil {
		return nil, fmt.Errorf("redis: cannot connect to REDIS endpoint: %s %v", address, err)
	}

	return &redisLogger{
		client:    redisClient,
		ctx:       ctx,
		key:       redisKey,
		tag:       tag,
		container: string(containerName),
		hostname:  hostname,
		extra:     ctx.ExtraAttributes(nil),
	}, nil
}

func (l *redisLogger) Log(msg *logger.Message) error {

	m, err := json.Marshal(message{
		string(msg.Line),
		l.container,
		l.hostname,
		l.tag,
		l.extra,
	})
	if err != nil {
		return err
	}

	if err := l.client.RPush(l.key, string(m)).Err(); err != nil {
		return fmt.Errorf("redis: cannot send REDIS message: %v", err)
	}

	return nil
}

func (l *redisLogger) Close() error {
	return l.client.Close()
}

func (l *redisLogger) Name() string {
	return name
}

// ValidateLogOpt looks for redis specific log options redis-address, redis-key, redis-password &
// redis-database.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "redis-address":
		case "redis-password":
		case "redis-database":
		case "redis-key":
		case "tag":
		case "labels":
		case "env":
		default:
			return fmt.Errorf("unknown log opt '%s' for redis log driver", key)
		}
	}

	if _, err := parseAddress(cfg["redis-address"]); err != nil {
		return err
	}

	return nil
}

func parseAddress(address string) (string, error) {
	if address == "" {
		return "", nil
	}
	if !urlutil.IsTransportURL(address) {
		return "", fmt.Errorf("redis-address should be in form protocol://address, got %v", address)
	}
	url, err := url.Parse(address)
	if err != nil {
		return "", err
	}

	// get host and port
	if _, _, err = net.SplitHostPort(url.Host); err != nil {
		return "", fmt.Errorf("redis: please provide redis-address as protocol://host:port")
	}

	return url.Host, nil
}
