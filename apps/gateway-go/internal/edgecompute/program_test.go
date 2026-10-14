package edgecompute

import (
	"fmt"
	"strings"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/state"
)

func TestCompileAndEvaluateIndustrialExpression(t *testing.T) {
	cfg := testConfig("clamp(max(a + b, 10.0), 0.0, 100.0) % 7.0", "float64")
	program, err := Compile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	point := program.Points[0]
	out, _, err := point.Program.Eval(map[string]interface{}{"a": 8.0, "b": 5.0})
	if err != nil {
		t.Fatal(err)
	}
	value, err := ConvertOutput("float64", out.Value())
	if err != nil {
		t.Fatal(err)
	}
	if value != float64(6) {
		t.Fatalf("value = %v, want 6", value)
	}
}

func TestNaturalIntegerConstantsWorkWithCollectedNumericValues(t *testing.T) {
	tests := []struct {
		expression string
		want       interface{}
	}{
		{"a + 1", float64(9)},
		{"a - 1", float64(7)},
		{"a * 2", float64(16)},
		{"a / 2", float64(4)},
		{"a % 3", float64(2)},
		{"min(a, 10)", float64(8)},
		{"max(a, 10)", float64(10)},
		{"clamp(a, 0, 100)", float64(8)},
		{"a > 7", true},
		{"a == 8", true},
		{"a != 7", true},
	}
	for _, test := range tests {
		outputType := "float64"
		if _, ok := test.want.(bool); ok {
			outputType = "bool"
		}
		cfg := testConfig(test.expression, outputType)
		program, err := Compile(cfg)
		if err != nil {
			t.Fatalf("Compile(%q): %v", test.expression, err)
		}
		out, _, err := program.Points[0].Program.Eval(map[string]interface{}{"a": 8.0, "b": 5.0})
		if err != nil {
			t.Fatalf("Eval(%q): %v", test.expression, err)
		}
		value, err := ConvertOutput(outputType, out.Value())
		if err != nil {
			t.Fatalf("ConvertOutput(%q): %v", test.expression, err)
		}
		if value != test.want {
			t.Fatalf("%s = %v, want %v", test.expression, value, test.want)
		}
	}
}

func TestBooleanEqualityRemainsAvailable(t *testing.T) {
	cfg := config.Config{GatewayKey: "gw", Points: []config.PointConfig{{DeviceKey: "source", Metric: "flag", DataType: "bool"}}, EdgeComputing: config.EdgeComputingConfig{Enabled: true, Groups: []config.EdgeComputeGroupConfig{{GroupKey: "edge", Points: []config.EdgeComputedPointConfig{{
		Name: "result", Metric: "result", DataType: "bool", Expression: "flag == true",
		Inputs: []config.EdgeComputeInputConfig{{Alias: "flag", SourceDeviceKey: "source", SourceMetric: "flag"}},
	}}}}}}
	program, err := Compile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := program.Points[0].Program.Eval(map[string]interface{}{"flag": true})
	if err != nil || out.Value() != true {
		t.Fatalf("value=%v err=%v", out.Value(), err)
	}
}

func TestPreviewUsesCurrentStateWithoutMutatingDerivedPoint(t *testing.T) {
	cfg := testConfig("a + b", "float64")
	cfg.ApplyDefaults()
	store := state.New(cfg)
	store.SetPointValue("source", "a", 2.5)
	store.SetPointValue("source", "b", 3.5)
	value, inputs, err := Preview(cfg, store, "edge", "result")
	if err != nil {
		t.Fatal(err)
	}
	if value != float64(6) || inputs["a"] != float64(2.5) || inputs["b"] != float64(3.5) {
		t.Fatalf("value=%v inputs=%v", value, inputs)
	}
	for _, status := range store.PointStatuses() {
		if status.DeviceKey == "edge" && status.Metric == "result" && status.UpdatedAt != nil {
			t.Fatal("preview mutated derived point state")
		}
	}
}

func TestCompileRejectsCycleAndMissingInput(t *testing.T) {
	cfg := config.Config{GatewayKey: "gw", EdgeComputing: config.EdgeComputingConfig{Enabled: true, Groups: []config.EdgeComputeGroupConfig{{
		GroupKey: "edge", Points: []config.EdgeComputedPointConfig{
			{Name: "A", Metric: "a", DataType: "float64", Expression: "b + 1.0", Inputs: []config.EdgeComputeInputConfig{{Alias: "b", SourceDeviceKey: "edge", SourceMetric: "b"}}},
			{Name: "B", Metric: "b", DataType: "float64", Expression: "a + 1.0", Inputs: []config.EdgeComputeInputConfig{{Alias: "a", SourceDeviceKey: "edge", SourceMetric: "a"}}},
		},
	}}}}
	if _, err := Compile(cfg); err == nil || !strings.Contains(err.Error(), "循环依赖") {
		t.Fatalf("cycle error = %v", err)
	}
	cfg.EdgeComputing.Groups[0].Points = cfg.EdgeComputing.Groups[0].Points[:1]
	cfg.EdgeComputing.Groups[0].Points[0].Inputs[0].SourceMetric = "missing"
	if _, err := Compile(cfg); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("missing input error = %v", err)
	}
}

func TestDisabledDraftDoesNotBlockEnabledDerivedPoint(t *testing.T) {
	disabled := false
	cfg := testConfig("a + b", "float64")
	cfg.EdgeComputing.Groups = append(cfg.EdgeComputing.Groups, config.EdgeComputeGroupConfig{
		GroupKey: "draft-group",
		Enabled:  &disabled,
		Points: []config.EdgeComputedPointConfig{{
			Name: "unfinished", Metric: "unfinished", DataType: "float64",
		}},
	})
	program, err := Compile(cfg)
	if err != nil {
		t.Fatalf("disabled draft blocked enabled rule: %v", err)
	}
	if len(program.Points) != 1 || program.Points[0].Key != "edge::result" {
		t.Fatalf("compiled points = %#v, want only edge::result", program.Points)
	}
}

func TestEnabledPointCannotDependOnDisabledDerivedPoint(t *testing.T) {
	disabled := false
	cfg := testConfig("a + b", "float64")
	cfg.EdgeComputing.Groups[0].Points = append(cfg.EdgeComputing.Groups[0].Points,
		config.EdgeComputedPointConfig{
			Name: "draft", Metric: "draft", Enabled: &disabled, DataType: "float64",
		},
		config.EdgeComputedPointConfig{
			Name: "consumer", Metric: "consumer", DataType: "float64", Expression: "draft + 1.0",
			Inputs: []config.EdgeComputeInputConfig{{Alias: "draft", SourceDeviceKey: "edge", SourceMetric: "draft"}},
		},
	)
	if _, err := Compile(cfg); err == nil || !strings.Contains(err.Error(), "不存在的输入") {
		t.Fatalf("disabled dependency error = %v", err)
	}
}

func TestIntegerOutputRequiresIntegralInRangeValue(t *testing.T) {
	if _, err := ConvertOutput("uint16", 1.5); err == nil {
		t.Fatal("fractional integer output accepted")
	}
	if _, err := ConvertOutput("uint16", -1.0); err == nil {
		t.Fatal("negative uint16 output accepted")
	}
	if value, err := ConvertOutput("uint16", 42.0); err != nil || value != uint16(42) {
		t.Fatalf("value=%v err=%v", value, err)
	}
}

func TestExpressionSurfaceIsRestricted(t *testing.T) {
	if _, err := Compile(testConfig("a > b ? abs(a) : round(b)", "float64")); err != nil {
		t.Fatalf("supported conditional expression rejected: %v", err)
	}
	for _, expression := range []string{"string(a)", "[a, b].map(x, x * 2.0)", "size([a, b])"} {
		if _, err := Compile(testConfig(expression, "float64")); err == nil {
			t.Fatalf("unsafe or unsupported expression accepted: %s", expression)
		}
	}
}

func BenchmarkCompileAndEvaluate1000Rules(b *testing.B) {
	cfg := config.Config{GatewayKey: "gw", Points: []config.PointConfig{{DeviceKey: "source", Metric: "x", DataType: "float64"}}, EdgeComputing: config.EdgeComputingConfig{Enabled: true}}
	group := config.EdgeComputeGroupConfig{GroupKey: "edge"}
	for index := 0; index < 1000; index++ {
		group.Points = append(group.Points, config.EdgeComputedPointConfig{Name: fmt.Sprint(index), Metric: fmt.Sprintf("p%d", index), DataType: "float64", Expression: "x * 2.0", Inputs: []config.EdgeComputeInputConfig{{Alias: "x", SourceDeviceKey: "source", SourceMetric: "x"}}})
	}
	cfg.EdgeComputing.Groups = []config.EdgeComputeGroupConfig{group}
	program, err := Compile(cfg)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for _, point := range program.Points {
			if _, _, err := point.Program.Eval(map[string]interface{}{"x": 12.0}); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func testConfig(expression, outputType string) config.Config {
	return config.Config{GatewayKey: "gw", Points: []config.PointConfig{
		{DeviceKey: "source", Metric: "a", DataType: "float64"},
		{DeviceKey: "source", Metric: "b", DataType: "float64"},
	}, EdgeComputing: config.EdgeComputingConfig{Enabled: true, Groups: []config.EdgeComputeGroupConfig{{GroupKey: "edge", Points: []config.EdgeComputedPointConfig{{
		Name: "result", Metric: "result", DataType: outputType, Expression: expression,
		Inputs: []config.EdgeComputeInputConfig{{Alias: "a", SourceDeviceKey: "source", SourceMetric: "a"}, {Alias: "b", SourceDeviceKey: "source", SourceMetric: "b"}},
	}}}}}}
}
