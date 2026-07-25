package config

import "time"

var StartTime = time.Now().Unix() // unit: second

// Version is replaced during build; keep the source default stable.
var Version = "v0.0.0"

var SystemName = "codego-api"
