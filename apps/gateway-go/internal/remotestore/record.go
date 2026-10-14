package remotestore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"weikong-iot-platform/apps/gateway-go/internal/state"
)

type Record struct {
	ID         string   `json:"id"`
	Gateway    string   `json:"gateway"`
	Device     string   `json:"device"`
	Metric     string   `json:"metric"`
	Sampled    int64    `json:"sampled"`
	Received   *int64   `json:"received,omitempty"`
	Source     *int64   `json:"source,omitempty"`
	Type       string   `json:"type"`
	ValueJSON  string   `json:"valueJson"`
	Integer    *string  `json:"integer,omitempty"`
	Float      *float64 `json:"float,omitempty"`
	Boolean    *int     `json:"boolean,omitempty"`
	Text       *string  `json:"text,omitempty"`
	ActualType string   `json:"actualType"`
	Quality    string   `json:"quality"`
	Unit       string   `json:"unit"`
	Diagnostic string   `json:"diagnostic"`
}

func ms(t *time.Time) *int64 {
	if t == nil || t.IsZero() {
		return nil
	}
	v := t.UnixMilli()
	return &v
}
func NewRecord(gateway string, p state.PointStatus, at time.Time) (Record, error) {
	for _, field := range []struct {
		value string
		limit int
	}{{gateway, 256}, {p.DeviceKey, 256}, {p.Metric, 256}, {p.ActualDataType, 128}, {p.Unit, 128}, {string(p.Quality), 24}} {
		if !utf8.ValidString(field.value) || strings.ContainsRune(field.value, 0) || len(field.value) > field.limit {
			return Record{}, fmt.Errorf("采样标识、类型或单位超出专用表长度限制或包含无效字符，未入队")
		}
	}
	var id [24]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Record{}, err
	}
	r := Record{ID: hex.EncodeToString(id[:]), Gateway: gateway, Device: p.DeviceKey, Metric: p.Metric, Sampled: at.UnixMilli(), Received: ms(p.ReceiveTimestamp), Source: ms(p.SourceTimestamp), ActualType: p.ActualDataType, Quality: string(p.Quality), Unit: p.Unit, Diagnostic: p.Diagnostic}
	if r.Received == nil {
		r.Received = ms(p.UpdatedAt)
	}
	if p.Stale {
		r.Quality = "OFFLINE"
	}
	if p.Error != "" {
		r.Quality = "BAD"
		r.Diagnostic = p.Error
	}
	raw, err := json.Marshal(p.Value)
	if err != nil {
		raw, _ = json.Marshal(fmt.Sprint(p.Value))
		r.Type = "special"
	} else {
		r.Type = "json"
	}
	r.ValueJSON = string(raw)
	if p.Value == nil {
		r.Type = "null"
		if r.Quality == "" {
			r.Quality = "BAD"
			r.Diagnostic = "尚未取得点位值"
		}
	} else if n, ok := p.Value.(json.Number); ok {
		// JSON-decoded int64/uint64 values must not round-trip through float64.
		s := n.String()
		if _, e := strconv.ParseInt(s, 10, 64); e == nil {
			r.Type, r.Integer = "integer", &s
		} else if _, e := strconv.ParseUint(s, 10, 64); e == nil {
			r.Type, r.Integer = "integer", &s
		} else if f, e := n.Float64(); e == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
			r.Type, r.Float = "float", &f
		}
	} else {
		v := reflect.ValueOf(p.Value)
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			s := strconv.FormatInt(v.Int(), 10)
			r.Type = "integer"
			r.Integer = &s
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			s := strconv.FormatUint(v.Uint(), 10)
			r.Type = "integer"
			r.Integer = &s
		case reflect.Float32, reflect.Float64:
			n := v.Float()
			if !math.IsNaN(n) && !math.IsInf(n, 0) {
				r.Type = "float"
				r.Float = &n
			}
		case reflect.Bool:
			n := 0
			if v.Bool() {
				n = 1
			}
			r.Type = "boolean"
			r.Boolean = &n
		case reflect.String:
			s := v.String()
			r.Type = "string"
			r.Text = &s
		}
	}
	data, _ := json.Marshal(r)
	// PostgreSQL text cannot contain NUL; reject unrepresentable text before
	// it becomes a poison record blocking all following durable samples.
	if strings.ContainsRune(r.Diagnostic, 0) || (r.Text != nil && strings.ContainsRune(*r.Text, 0)) {
		return Record{}, fmt.Errorf("文本值或诊断包含 NUL 字符，未入队")
	}
	if len(data) > 64*1024 {
		return Record{}, fmt.Errorf("单条采样超过 64KB，未入队")
	}
	return r, nil
}
func (r Record) Args() []any {
	return []any{r.ID, r.Gateway, r.Device, r.Metric, r.Sampled, r.Received, r.Source, r.Type, r.ValueJSON, r.Integer, r.Float, r.Boolean, r.Text, r.ActualType, r.Quality, r.Unit, r.Diagnostic}
}
