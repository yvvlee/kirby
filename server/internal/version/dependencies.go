//go:build deps

// Package version reports build information for the Kirby server.
package version

// This file anchors the public modules reserved by the extraction plan. Later
// tasks use these packages without changing the shared dependency manifest.

import (
	"github.com/envoyproxy/protoc-gen-validate/validate"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/argon2"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
	"xorm.io/xorm"
)

var (
	_ = annotations.E_Http
	_ = argon2.IDKey
	_ = grpc.NewServer
	_ = jwt.New
	_ = kratos.New
	_ = minio.New
	_ = mysql.MySQLDriver{}
	_ = proto.Marshal
	_ = rate.NewLimiter
	_ = redis.NewClient
	_ = singleflight.Group{}
	_ = uuid.New
	_ = validate.E_Disabled
	_ = wire.Build
	_ = xorm.NewEngine
	_ = yaml.Unmarshal
)
