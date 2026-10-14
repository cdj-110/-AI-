package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

// PointQuality is the protocol-neutral quality exposed by the gateway.  The
// raw protocol quality is retained separately so downstream protocols and
// diagnostics do not have to reverse-map this normalized value.
type PointQuality string

const (
	QualityGood      PointQuality = "GOOD"
	QualityUncertain PointQuality = "UNCERTAIN"
	QualityBad       PointQuality = "BAD"
	QualityOffline   PointQuality = "OFFLINE"
)

type Reading struct {
	DeviceKey string                 `json:"deviceKey"`
	Time      time.Time              `json:"time"`
	Metrics   map[string]interface{} `json:"metrics"`
}

type PointValue struct {
	DeviceKey        string
	Metric           string
	Value            interface{}
	ActualDataType   string
	Quality          PointQuality
	RawQuality       uint32
	SourceTimestamp  time.Time
	ReceiveTimestamp time.Time
	CollectionMode   string
	DiagnosticCode   int64
	Diagnostic       string
}

// ApplyDefaults keeps existing collectors backward compatible while allowing
// protocols such as OPC DA to provide source quality and timestamp metadata.
func (p *PointValue) ApplyDefaults(now time.Time) {
	if p.Quality == "" {
		p.Quality = QualityGood
	}
	if p.ReceiveTimestamp.IsZero() {
		p.ReceiveTimestamp = now
	}
	if p.ActualDataType == "" || strings.EqualFold(p.ActualDataType, "auto") {
		p.ActualDataType = InferDataType(p.Value)
	}
}

// InferDataType provides a common fallback for protocols that do not expose a
// native wire type. Drivers that do expose it (for example OPC DA VARIANT or
// OPC UA built-in types) keep their more precise ActualDataType unchanged.
func InferDataType(value interface{}) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case string:
		return "string"
	case int:
		return "int"
	case int8:
		return "int8"
	case int16:
		return "int16"
	case int32:
		return "int32"
	case int64:
		return "int64"
	case uint:
		return "uint"
	case uint8:
		return "uint8"
	case uint16:
		return "uint16"
	case uint32:
		return "uint32"
	case uint64:
		return "uint64"
	case float32:
		return "float32"
	case float64:
		return "float64"
	case json.Number:
		return "number"
	case time.Time:
		return "datetime"
	}
	typeOf := reflect.TypeOf(value)
	if typeOf != nil && (typeOf.Kind() == reflect.Slice || typeOf.Kind() == reflect.Array) {
		return inferReflectType(typeOf.Elem()) + "[]"
	}
	return "json"
}

func inferReflectType(value reflect.Type) string {
	switch value.Kind() {
	case reflect.Bool:
		return "bool"
	case reflect.String:
		return "string"
	case reflect.Int8:
		return "int8"
	case reflect.Int16:
		return "int16"
	case reflect.Int32:
		return "int32"
	case reflect.Int, reflect.Int64:
		return strings.ToLower(value.Kind().String())
	case reflect.Uint8:
		return "uint8"
	case reflect.Uint16:
		return "uint16"
	case reflect.Uint32:
		return "uint32"
	case reflect.Uint, reflect.Uint64:
		return strings.ToLower(value.Kind().String())
	case reflect.Float32:
		return "float32"
	case reflect.Float64:
		return "float64"
	case reflect.Slice, reflect.Array:
		return inferReflectType(value.Elem()) + "[]"
	default:
		return "json"
	}
}
