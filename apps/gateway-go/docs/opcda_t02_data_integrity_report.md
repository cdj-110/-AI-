# OPC DA T02 数据完整性测试报告

测试日期：2026-08-26  
目标设备：EG100 ARMv7  
实机服务器：Matrikon OPC Simulation，Packet Privacy

## 结论

标量 VARIANT 解码测试通过。整数保持符号与位宽，UINT32 最大值不会变成 -1，
INT64/UINT64 在原生层和 Go 状态层保持精确值；Web 状态额外提供 `valueText`，
避免 JavaScript Number 对大于 2^53 的整数造成显示精度丢失。

SAFEARRAY 已支持 `VT_ARRAY | VT_R8` 和 `VT_ARRAY | VT_BSTR`，并会以 `float64[]` / `string[]` 保留数组边界内的全部元素。其他 VT_ARRAY 类型仍会明确返回不支持错误，不会将其误报为 GOOD。

## 确定性边界测试

| 类型 | 测试值 | 结果 |
| --- | --- | --- |
| VT_I1 / VT_UI1 | -128 / 255 | 通过 |
| VT_I2 | -32768 / 0 / 32767 | 通过 |
| VT_UI2 | 0 / 65535 | 通过 |
| VT_I4 | -2147483648 / 2147483647 | 通过 |
| VT_UI4 | 4294967295 | 通过，未转换为 -1 |
| VT_I8 | -9223372036854775808 / 9223372036854775807 | 通过 |
| VT_UI8 | 18446744073709551615 | 通过 |
| VT_R4 / VT_R8 | 123.25 / -9876.5 | 通过 |
| VT_BOOL | VARIANT_TRUE(-1) / VARIANT_FALSE(0) | 通过 |
| VT_DATE | 25570.5（1970-01-02 12:00:00 UTC） | 通过 |
| VT_BSTR | `ABC123` | 通过 |
| VT_BSTR | `中文测试` | 通过，无乱码 |
| VT_BSTR | 空字符串 | 通过 |
| VT_ARRAY | VT_ARRAY\|VT_I4 | 当前不支持，返回明确错误 |

相同测试分别在开发机和 EG100 ARMv7 上运行，均通过。

## Matrikon 实机读取

| Item ID | Server VARTYPE | EG100 类型 | 结果 |
| --- | ---: | --- | --- |
| Random.Real8 | VT_R8 (5) | floating_point | GOOD，HRESULT 0 |
| Random.Real4 | VT_R4 (4) | floating_point | GOOD，HRESULT 0 |
| Random.Int1 | VT_I1 (16) | signed_integer | GOOD，HRESULT 0 |
| Random.Int8 | VT_I8 (20) | signed_integer | GOOD，HRESULT 0 |
| Random.String | VT_BSTR (8) | string | GOOD，HRESULT 0，无乱码 |

实机五项均返回原始质量码 `0x00C0`，并包含有效源时间。

## 尚未完成的实机覆盖

当前 Matrikon 工程没有提供固定边界值、BOOL、DATE、UINT 和 ARRAY 测试标签，
这些类型已完成 ARMv7 确定性线协议测试，但还不能标记为“Server 到 EG100 全链路实机边界测试”。
如需完成该层验收，应在 OPC Server 新增固定值测试标签后再次读取并逐项对照。
