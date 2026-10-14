package collector

import "strings"

// DLT645DataItem describes one portable data item. DI values are kept in the
// human-readable order used by meter manuals; frame encoding reverses bytes.
type DLT645DataItem struct {
	Key         string `json:"key"`
	Category    string `json:"category"`
	Name        string `json:"name"`
	DI2007      string `json:"di2007,omitempty"`
	DI1997      string `json:"di1997,omitempty"`
	ValueFormat string `json:"valueFormat"`
	DataLength  int    `json:"dataLength"`
	Decimals    int    `json:"decimals"`
	Unit        string `json:"unit,omitempty"`
	DataType    string `json:"dataType"`
	Access      string `json:"access"`
}

var dlt645DataItems = []DLT645DataItem{
	{Key: "combined-active-total", Category: "电能", Name: "当前组合有功总电能", DI2007: "00000000", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "combined-active-rate-1", Category: "电能", Name: "当前组合有功费率1电能", DI2007: "00000100", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "combined-active-rate-2", Category: "电能", Name: "当前组合有功费率2电能", DI2007: "00000200", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "combined-active-rate-3", Category: "电能", Name: "当前组合有功费率3电能", DI2007: "00000300", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "combined-active-rate-4", Category: "电能", Name: "当前组合有功费率4电能", DI2007: "00000400", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "forward-active-total", Category: "电能", Name: "当前正向有功总电能", DI2007: "00010000", DI1997: "9010", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "forward-active-rate-1", Category: "电能", Name: "当前正向有功费率1电能", DI2007: "00010100", DI1997: "9011", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "forward-active-rate-2", Category: "电能", Name: "当前正向有功费率2电能", DI2007: "00010200", DI1997: "9012", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "forward-active-rate-3", Category: "电能", Name: "当前正向有功费率3电能", DI2007: "00010300", DI1997: "9013", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "forward-active-rate-4", Category: "电能", Name: "当前正向有功费率4电能", DI2007: "00010400", DI1997: "9014", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "reverse-active-total", Category: "电能", Name: "当前反向有功总电能", DI2007: "00020000", DI1997: "9110", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "reverse-active-rate-1", Category: "电能", Name: "当前反向有功费率1电能", DI2007: "00020100", DI1997: "9111", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "reverse-active-rate-2", Category: "电能", Name: "当前反向有功费率2电能", DI2007: "00020200", DI1997: "9112", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "reverse-active-rate-3", Category: "电能", Name: "当前反向有功费率3电能", DI2007: "00020300", DI1997: "9113", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "reverse-active-rate-4", Category: "电能", Name: "当前反向有功费率4电能", DI2007: "00020400", DI1997: "9114", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kWh", DataType: "float64", Access: "read"},
	{Key: "forward-reactive-total", Category: "电能", Name: "当前组合无功1总电能", DI2007: "00030000", DI1997: "9410", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kvarh", DataType: "float64", Access: "read"},
	{Key: "forward-reactive-rate-1", Category: "电能", Name: "当前组合无功1费率1电能", DI2007: "00030100", DI1997: "9411", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kvarh", DataType: "float64", Access: "read"},
	{Key: "forward-reactive-rate-2", Category: "电能", Name: "当前组合无功1费率2电能", DI2007: "00030200", DI1997: "9412", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kvarh", DataType: "float64", Access: "read"},
	{Key: "forward-reactive-rate-3", Category: "电能", Name: "当前组合无功1费率3电能", DI2007: "00030300", DI1997: "9413", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kvarh", DataType: "float64", Access: "read"},
	{Key: "forward-reactive-rate-4", Category: "电能", Name: "当前组合无功1费率4电能", DI2007: "00030400", DI1997: "9414", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kvarh", DataType: "float64", Access: "read"},
	{Key: "reverse-reactive-total", Category: "电能", Name: "当前组合无功2总电能", DI2007: "00040000", DI1997: "9510", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kvarh", DataType: "float64", Access: "read"},
	{Key: "reverse-reactive-rate-1", Category: "电能", Name: "当前组合无功2费率1电能", DI2007: "00040100", DI1997: "9511", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kvarh", DataType: "float64", Access: "read"},
	{Key: "reverse-reactive-rate-2", Category: "电能", Name: "当前组合无功2费率2电能", DI2007: "00040200", DI1997: "9512", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kvarh", DataType: "float64", Access: "read"},
	{Key: "reverse-reactive-rate-3", Category: "电能", Name: "当前组合无功2费率3电能", DI2007: "00040300", DI1997: "9513", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kvarh", DataType: "float64", Access: "read"},
	{Key: "reverse-reactive-rate-4", Category: "电能", Name: "当前组合无功2费率4电能", DI2007: "00040400", DI1997: "9514", ValueFormat: "bcd", DataLength: 4, Decimals: 2, Unit: "kvarh", DataType: "float64", Access: "read"},
	{Key: "forward-active-demand", Category: "需量", Name: "当前正向有功总最大需量及发生时间", DI2007: "01010000", DI1997: "A010", ValueFormat: "demand-time", DataLength: 8, Decimals: 4, Unit: "kW", DataType: "object", Access: "read"},
	{Key: "reverse-active-demand", Category: "需量", Name: "当前反向有功总最大需量及发生时间", DI2007: "01020000", DI1997: "A020", ValueFormat: "demand-time", DataLength: 8, Decimals: 4, Unit: "kW", DataType: "object", Access: "read"},
	{Key: "voltage-a", Category: "瞬时量", Name: "A相电压", DI2007: "02010100", DI1997: "B611", ValueFormat: "bcd", DataLength: 2, Decimals: 1, Unit: "V", DataType: "float64", Access: "read"},
	{Key: "voltage-b", Category: "瞬时量", Name: "B相电压", DI2007: "02010200", DI1997: "B612", ValueFormat: "bcd", DataLength: 2, Decimals: 1, Unit: "V", DataType: "float64", Access: "read"},
	{Key: "voltage-c", Category: "瞬时量", Name: "C相电压", DI2007: "02010300", DI1997: "B613", ValueFormat: "bcd", DataLength: 2, Decimals: 1, Unit: "V", DataType: "float64", Access: "read"},
	{Key: "current-a", Category: "瞬时量", Name: "A相电流", DI2007: "02020100", DI1997: "B621", ValueFormat: "bcd", DataLength: 3, Decimals: 3, Unit: "A", DataType: "float64", Access: "read"},
	{Key: "current-b", Category: "瞬时量", Name: "B相电流", DI2007: "02020200", DI1997: "B622", ValueFormat: "bcd", DataLength: 3, Decimals: 3, Unit: "A", DataType: "float64", Access: "read"},
	{Key: "current-c", Category: "瞬时量", Name: "C相电流", DI2007: "02020300", DI1997: "B623", ValueFormat: "bcd", DataLength: 3, Decimals: 3, Unit: "A", DataType: "float64", Access: "read"},
	{Key: "active-power-total", Category: "瞬时量", Name: "总有功功率", DI2007: "02030000", DI1997: "B630", ValueFormat: "signed-bcd", DataLength: 3, Decimals: 4, Unit: "kW", DataType: "float64", Access: "read"},
	{Key: "active-power-a", Category: "瞬时量", Name: "A相有功功率", DI2007: "02030100", DI1997: "B631", ValueFormat: "signed-bcd", DataLength: 3, Decimals: 4, Unit: "kW", DataType: "float64", Access: "read"},
	{Key: "active-power-b", Category: "瞬时量", Name: "B相有功功率", DI2007: "02030200", DI1997: "B632", ValueFormat: "signed-bcd", DataLength: 3, Decimals: 4, Unit: "kW", DataType: "float64", Access: "read"},
	{Key: "active-power-c", Category: "瞬时量", Name: "C相有功功率", DI2007: "02030300", DI1997: "B633", ValueFormat: "signed-bcd", DataLength: 3, Decimals: 4, Unit: "kW", DataType: "float64", Access: "read"},
	{Key: "reactive-power-total", Category: "瞬时量", Name: "总无功功率", DI2007: "02040000", DI1997: "B640", ValueFormat: "signed-bcd", DataLength: 3, Decimals: 4, Unit: "kvar", DataType: "float64", Access: "read"},
	{Key: "reactive-power-a", Category: "瞬时量", Name: "A相无功功率", DI2007: "02040100", DI1997: "B641", ValueFormat: "signed-bcd", DataLength: 3, Decimals: 4, Unit: "kvar", DataType: "float64", Access: "read"},
	{Key: "reactive-power-b", Category: "瞬时量", Name: "B相无功功率", DI2007: "02040200", DI1997: "B642", ValueFormat: "signed-bcd", DataLength: 3, Decimals: 4, Unit: "kvar", DataType: "float64", Access: "read"},
	{Key: "reactive-power-c", Category: "瞬时量", Name: "C相无功功率", DI2007: "02040300", DI1997: "B643", ValueFormat: "signed-bcd", DataLength: 3, Decimals: 4, Unit: "kvar", DataType: "float64", Access: "read"},
	{Key: "apparent-power-total", Category: "瞬时量", Name: "总视在功率", DI2007: "02050000", DI1997: "B660", ValueFormat: "bcd", DataLength: 3, Decimals: 4, Unit: "kVA", DataType: "float64", Access: "read"},
	{Key: "apparent-power-a", Category: "瞬时量", Name: "A相视在功率", DI2007: "02050100", DI1997: "B661", ValueFormat: "bcd", DataLength: 3, Decimals: 4, Unit: "kVA", DataType: "float64", Access: "read"},
	{Key: "apparent-power-b", Category: "瞬时量", Name: "B相视在功率", DI2007: "02050200", DI1997: "B662", ValueFormat: "bcd", DataLength: 3, Decimals: 4, Unit: "kVA", DataType: "float64", Access: "read"},
	{Key: "apparent-power-c", Category: "瞬时量", Name: "C相视在功率", DI2007: "02050300", DI1997: "B663", ValueFormat: "bcd", DataLength: 3, Decimals: 4, Unit: "kVA", DataType: "float64", Access: "read"},
	{Key: "power-factor-total", Category: "瞬时量", Name: "总功率因数", DI2007: "02060000", DI1997: "B650", ValueFormat: "signed-bcd", DataLength: 2, Decimals: 3, DataType: "float64", Access: "read"},
	{Key: "power-factor-a", Category: "瞬时量", Name: "A相功率因数", DI2007: "02060100", DI1997: "B651", ValueFormat: "signed-bcd", DataLength: 2, Decimals: 3, DataType: "float64", Access: "read"},
	{Key: "power-factor-b", Category: "瞬时量", Name: "B相功率因数", DI2007: "02060200", DI1997: "B652", ValueFormat: "signed-bcd", DataLength: 2, Decimals: 3, DataType: "float64", Access: "read"},
	{Key: "power-factor-c", Category: "瞬时量", Name: "C相功率因数", DI2007: "02060300", DI1997: "B653", ValueFormat: "signed-bcd", DataLength: 2, Decimals: 3, DataType: "float64", Access: "read"},
	{Key: "frequency", Category: "瞬时量", Name: "电网频率", DI2007: "02800002", DI1997: "B680", ValueFormat: "bcd", DataLength: 2, Decimals: 2, Unit: "Hz", DataType: "float64", Access: "read"},
	{Key: "meter-date", Category: "时间", Name: "电表日期", DI2007: "04000101", DI1997: "C010", ValueFormat: "date", DataLength: 4, DataType: "string", Access: "read"},
	{Key: "meter-time", Category: "时间", Name: "电表时间", DI2007: "04000102", DI1997: "C011", ValueFormat: "time", DataLength: 3, DataType: "string", Access: "read"},
	{Key: "status-word-1", Category: "状态", Name: "电表运行状态字1", DI2007: "04000501", DI1997: "C020", ValueFormat: "binary", DataLength: 2, DataType: "uint16", Access: "read"},
	{Key: "status-word-2", Category: "状态", Name: "电表运行状态字2", DI2007: "04000502", DI1997: "C023", ValueFormat: "binary", DataLength: 2, DataType: "uint16", Access: "read"},
}

func DLT645Dictionary() []DLT645DataItem {
	return append([]DLT645DataItem(nil), dlt645DataItems...)
}

func findDLT645Item(key string) (DLT645DataItem, bool) {
	key = strings.TrimSpace(key)
	for _, item := range dlt645DataItems {
		if item.Key == key {
			return item, true
		}
	}
	return DLT645DataItem{}, false
}
