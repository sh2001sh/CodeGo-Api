package config

import (
	"crypto/tls"
	"time"

	"github.com/google/uuid"
)

var SessionSecret = uuid.New().String()
var CryptoSecret = uuid.New().String()

var DebugEnabled bool
var MemoryCacheEnabled bool
var IsMasterNode bool

// NodeName identifies the current node in audit logs and clustered deployments.
var NodeName string

var TLSInsecureSkipVerify bool
var InsecureTLSConfig = &tls.Config{InsecureSkipVerify: true}

var RequestInterval time.Duration
var SyncFrequency int
var BatchUpdateInterval int

var RelayTimeout int

// RelayResponseHeaderTimeout bounds outbound connection, TLS, and response-header waits
// without limiting a stream after the upstream has started responding.
var RelayResponseHeaderTimeout int
var RelayMaxIdleConns int
var RelayMaxIdleConnsPerHost int
var TrustedProxies []string
