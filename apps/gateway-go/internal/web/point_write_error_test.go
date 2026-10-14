package web

import (
	"errors"
	"strings"
	"testing"
)

func TestDescribePointWriteError(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		code       string
		want       string
		targetType string
		validRange string
	}{
		{"int8 range", `OPC DA int8 write value "151" is invalid or out of range: strconv.ParseInt: parsing "151": value out of range`, "opcda_integer_out_of_range", "输入值 151 超出范围", "int8", "-128～127"},
		{"int16 invalid", `OPC DA int16 write value "ABC" is invalid or out of range: strconv.ParseInt: parsing "ABC": invalid syntax`, "opcda_integer_invalid", "不是有效整数", "int16", "-32768～32767"},
		{"int32 range", `OPC DA int32 write value "2147483648" is invalid or out of range: strconv.ParseInt: value out of range`, "opcda_integer_out_of_range", "-2147483648～2147483647", "int32", "-2147483648～2147483647"},
		{"int64 range", `OPC DA int64 write value "9223372036854775808" is invalid or out of range: strconv.ParseInt: value out of range`, "opcda_integer_out_of_range", "-9223372036854775808～9223372036854775807", "int64", "-9223372036854775808～9223372036854775807"},
		{"uint8 range", `OPC DA uint8 write value "256" is invalid or out of range: strconv.ParseUint: value out of range`, "opcda_integer_out_of_range", "0～255", "uint8", "0～255"},
		{"uint16 range", `OPC DA uint16 write value "65536" is invalid or out of range: strconv.ParseUint: value out of range`, "opcda_integer_out_of_range", "0～65535", "uint16", "0～65535"},
		{"uint32 range", `OPC DA uint32 write value "4294967296" is invalid or out of range: strconv.ParseUint: value out of range`, "opcda_integer_out_of_range", "0～4294967295", "uint32", "0～4294967295"},
		{"uint64 range", `OPC DA uint64 write value "18446744073709551616" is invalid or out of range: strconv.ParseUint: value out of range`, "opcda_integer_out_of_range", "0～18446744073709551615", "uint64", "0～18446744073709551615"},
		{"float invalid", `OPC DA float32 write value "ABC" is invalid: strconv.ParseFloat: invalid syntax`, "opcda_float_invalid", "不是有效浮点数", "float32", ""},
		{"float64 non-finite", `OPC DA float64 write value "NaN" is invalid: non-finite number`, "opcda_float_invalid", "不是有限数值", "float64", ""},
		{"bool invalid", `OPC DA BOOL write value "maybe" must be true/false or 1/0`, "opcda_bool_invalid", "true/false 或 1/0", "bool", ""},
		{"generic opcda", `OPC DA SyncWrite itemId=Random.Int1 HRESULT=0x80070005`, "opcda_write_failed", "OPC DA 写入失败", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := describePointWriteError(errors.New(test.raw))
			if got.OK || got.Code != test.code || got.TargetType != test.targetType || got.ValidRange != test.validRange {
				t.Fatalf("describePointWriteError() = %#v", got)
			}
			if !strings.Contains(got.Message, test.want) {
				t.Fatalf("message %q does not contain %q", got.Message, test.want)
			}
			if got.Detail != test.raw {
				t.Fatalf("detail = %q, want original %q", got.Detail, test.raw)
			}
		})
	}
}
