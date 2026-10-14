package config

import "testing"

func TestDLT645DefaultsAndValidation(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gateway","points":[{"deviceKey":"meter-1","metric":"dlt645-P01","protocol":"dlt645","address":"/dev/ttyS1","dlt645":{"meterAddress":"000000000001","itemKey":"forward-active-total"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	point := cfg.Points[0]
	if point.BaudRate != 2400 || point.DataBits != 8 || point.Parity != "even" || point.StopBits != 1 {
		t.Fatalf("unexpected serial defaults: %d/%d/%s/%d", point.BaudRate, point.DataBits, point.Parity, point.StopBits)
	}
	if point.DLT645.Version != "auto" || point.DLT645.ResponseTimeoutMilliseconds != 1500 || point.DLT645.Retries != 1 || point.DLT645.WakeupBytes != 4 {
		t.Fatalf("unexpected protocol defaults: %#v", point.DLT645)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDLT645ExplicitZeroRetryAndWakeupArePreserved(t *testing.T) {
	cfg, err := Parse([]byte(`{"gatewayKey":"gateway","points":[{"deviceKey":"meter-1","metric":"dlt645-P01","protocol":"dlt645","address":"/dev/ttyS1","dlt645":{"meterAddress":"000000000001","itemKey":"forward-active-total","retries":0,"wakeupBytes":0}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Points[0].DLT645.Retries != 0 || cfg.Points[0].DLT645.WakeupBytes != 0 {
		t.Fatalf("explicit zero values changed: %#v", cfg.Points[0].DLT645)
	}
}

func TestDLT645RejectsInvalidAddressAndDI(t *testing.T) {
	valid, err := Parse([]byte(`{"gatewayKey":"gateway","points":[{"deviceKey":"meter-1","metric":"dlt645-P01","protocol":"dlt645","address":"/dev/ttyS1","dlt645":{"version":"2007","meterAddress":"000000000001","dataIdentifier":"00000000","valueFormat":"bcd","dataLength":4,"access":"read"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	base := valid.Points[0]
	if len(valid.Devices) > 0 && len(valid.Devices[0].Points) > 0 {
		base = valid.Devices[0].Points[0]
	}
	for name, mutate := range map[string]func(*PointConfig){
		"address":   func(point *PointConfig) { point.DLT645.MeterAddress = "123" },
		"broadcast": func(point *PointConfig) { point.DLT645.MeterAddress = "999999999999" },
		"di":        func(point *PointConfig) { point.DLT645.DataIdentifier = "1234" },
	} {
		t.Run(name, func(t *testing.T) {
			point := base
			options := *base.DLT645
			point.DLT645 = &options
			mutate(&point)
			candidate := valid
			candidate.Points = []PointConfig{point}
			if len(candidate.Devices) > 0 {
				candidate.Devices[0].Points = []PointConfig{point}
			}
			if err := candidate.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestDLT645SecretsAreMaskedAndPreserved(t *testing.T) {
	previous := Config{Devices: []DeviceConfig{{DeviceKey: "meter-1", Protocol: "dlt645", DLT645: &DLT645Config{WritePassword: "12345678", OperatorCode: "87654321"}}}}
	redacted := RedactSecrets(previous)
	if redacted.Devices[0].DLT645.WritePassword != "***" || redacted.Devices[0].DLT645.OperatorCode != "***" {
		t.Fatalf("credentials were not masked: %#v", redacted.Devices[0].DLT645)
	}
	PreserveMaskedSecrets(previous, &redacted)
	if redacted.Devices[0].DLT645.WritePassword != "12345678" || redacted.Devices[0].DLT645.OperatorCode != "87654321" {
		t.Fatalf("credentials were not restored: %#v", redacted.Devices[0].DLT645)
	}
}
