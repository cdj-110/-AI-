package web

import (
	"fmt"
	"regexp"
	"strings"
)

type pointWriteErrorBody struct {
	OK         bool   `json:"ok"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Detail     string `json:"detail"`
	TargetType string `json:"targetType,omitempty"`
	ValidRange string `json:"validRange,omitempty"`
}

var opcdaTypedWriteError = regexp.MustCompile(`(?i)OPC DA (int8|int16|int32|int64|int|uint8|uint16|uint32|uint64|uint|float32|float64|currency) write value "([^"]*)"`)
var opcdaBoolWriteError = regexp.MustCompile(`(?i)OPC DA BOOL write value "([^"]*)"`)
var opcdaDateWriteError = regexp.MustCompile(`(?i)OPC DA DATETIME write value "([^"]*)"`)

var opcdaIntegerRanges = map[string]string{
	"int8": "-128～127", "int16": "-32768～32767", "int32": "-2147483648～2147483647",
	"int": "-2147483648～2147483647", "int64": "-9223372036854775808～9223372036854775807",
	"uint8": "0～255", "uint16": "0～65535", "uint32": "0～4294967295",
	"uint": "0～4294967295", "uint64": "0～18446744073709551615",
}

func describePointWriteError(err error) pointWriteErrorBody {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	lower := strings.ToLower(detail)
	result := pointWriteErrorBody{
		OK: false, Code: "write_failed", Detail: detail,
		Message: "写入失败：请检查点位配置、目标值与设备连接后重试。",
	}

	if match := opcdaTypedWriteError.FindStringSubmatch(detail); len(match) == 3 {
		targetType, input := strings.ToLower(match[1]), match[2]
		result.TargetType = targetType
		if validRange, ok := opcdaIntegerRanges[targetType]; ok {
			result.ValidRange = validRange
			if strings.Contains(lower, "value out of range") {
				result.Code = "opcda_integer_out_of_range"
				result.Message = fmt.Sprintf("写入失败：目标数据类型为 %s，有效范围是 %s，输入值 %s 超出范围，请修改后重试。", targetType, validRange, input)
			} else {
				result.Code = "opcda_integer_invalid"
				result.Message = fmt.Sprintf("写入失败：目标数据类型为 %s，输入值 %q 不是有效整数；有效范围是 %s，请修改后重试。", targetType, input, validRange)
			}
			return result
		}
		result.Code = "opcda_float_invalid"
		if strings.Contains(lower, "non-finite") || strings.EqualFold(input, "nan") || strings.Contains(strings.ToLower(input), "inf") {
			result.Message = fmt.Sprintf("写入失败：目标数据类型为 %s，输入值 %q 不是有限数值，请输入有效浮点数后重试。", targetType, input)
		} else {
			result.Message = fmt.Sprintf("写入失败：目标数据类型为 %s，输入值 %q 不是有效浮点数，请修改后重试。", targetType, input)
		}
		return result
	}

	if match := opcdaBoolWriteError.FindStringSubmatch(detail); len(match) == 2 {
		result.Code = "opcda_bool_invalid"
		result.TargetType = "bool"
		result.Message = fmt.Sprintf("写入失败：目标数据类型为 bool，输入值 %q 无法识别；请输入 true/false 或 1/0。", match[1])
		return result
	}
	if match := opcdaDateWriteError.FindStringSubmatch(detail); len(match) == 2 {
		result.Code = "opcda_datetime_invalid"
		result.TargetType = "datetime"
		result.Message = fmt.Sprintf("写入失败：输入值 %q 不是有效时间；请输入 RFC3339、常用日期时间格式或 Unix 毫秒时间戳。", match[1])
		return result
	}
	if strings.Contains(lower, "array target type") {
		result.Code = "opcda_array_unsupported"
		result.Message = "写入失败：当前版本暂不支持 OPC DA 数组写入，请改用标量点位。"
		return result
	}
	if strings.Contains(lower, "does not support target type") {
		result.Code = "opcda_type_unsupported"
		result.Message = "写入失败：目标 OPC DA 数据类型当前不受支持，请检查点位请求类型。"
		return result
	}
	if strings.Contains(lower, "opc da") && (strings.Contains(lower, "write") || strings.Contains(lower, "syncwrite")) {
		result.Code = "opcda_write_failed"
		result.Message = "OPC DA 写入失败：请检查点位写权限、目标数据类型、服务器连接和 DCOM 安全配置。"
		return result
	}
	if strings.Contains(lower, "writable point not found") {
		result.Code = "point_not_writable"
		result.Message = "写入失败：未找到可写点位，请确认点位已启用且读写类型为“读写”或“只写”。"
		return result
	}
	if strings.Contains(lower, "does not support write") {
		result.Code = "protocol_write_unsupported"
		result.Message = "写入失败：当前协议或点位类型不支持写入。"
	}
	return result
}
