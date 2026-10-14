package edgecompute

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/google/cel-go/cel"
	celenv "github.com/google/cel-go/common/env"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

const (
	maxExpressionLength = 2048
	maxInputsPerPoint   = 64
	maxEvaluationCost   = 1000
)

var aliasPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,31}$`)

type CompiledPoint struct {
	Group      config.EdgeComputeGroupConfig
	Config     config.EdgeComputedPointConfig
	Key        string
	Signature  string
	Program    cel.Program
	InputTypes map[string]string
}

type Program struct {
	Config config.Config
	Points []*CompiledPoint
	ByKey  map[string]*CompiledPoint
}

func Validate(cfg config.Config) error {
	_, err := Compile(cfg)
	return err
}

func Compile(cfg config.Config) (*Program, error) {
	result := &Program{Config: cfg, ByKey: map[string]*CompiledPoint{}}
	if !cfg.EdgeComputing.Enabled {
		return result, nil
	}
	typesByKey := map[string]string{}
	physicalDeviceKeys := map[string]bool{}
	for _, point := range cfg.Points {
		key := sourceKey(point.DeviceKey, point.Metric)
		if key != "::" {
			typesByKey[key] = normalizedDataType(point.DataType)
			physicalDeviceKeys[point.DeviceKey] = true
		}
	}
	groupKeys := map[string]bool{}
	derivedKeys := map[string]bool{}
	for _, group := range cfg.EdgeComputing.Groups {
		if strings.TrimSpace(group.GroupKey) == "" {
			return nil, fmt.Errorf("边缘计算组标识不能为空")
		}
		if groupKeys[group.GroupKey] {
			return nil, fmt.Errorf("边缘计算组标识 %s 重复", group.GroupKey)
		}
		if physicalDeviceKeys[group.GroupKey] {
			return nil, fmt.Errorf("边缘计算组标识 %s 与采集设备标识重复", group.GroupKey)
		}
		if group.HeartbeatSeconds < 0 {
			return nil, fmt.Errorf("计算组 %s 的心跳周期不能小于 0", group.GroupKey)
		}
		groupKeys[group.GroupKey] = true
		for _, point := range group.Points {
			key := sourceKey(group.GroupKey, point.Metric)
			if point.Metric == "" {
				return nil, fmt.Errorf("计算组 %s 的派生点标识不能为空", group.GroupKey)
			}
			if derivedKeys[key] {
				return nil, fmt.Errorf("派生点 %s 重复", key)
			}
			derivedKeys[key] = true
			dataType := normalizedDataType(point.DataType)
			if !supportedOutputType(dataType) {
				return nil, fmt.Errorf("派生点 %s 的数据类型 %s 不受支持", key, point.DataType)
			}
			if point.Decimals < 0 || point.Decimals > 9 {
				return nil, fmt.Errorf("派生点 %s 的小数位必须为 0-9", key)
			}
			// A disabled rule is a retained draft and must not block unrelated,
			// enabled rules from running. Do not expose it as a valid dependency:
			// an enabled rule referencing a disabled output should fail clearly.
			if group.IsEnabled() && point.IsEnabled() {
				typesByKey[key] = dataType
			}
		}
	}

	for _, group := range cfg.EdgeComputing.Groups {
		if !group.IsEnabled() {
			continue
		}
		for _, point := range group.Points {
			if !point.IsEnabled() {
				continue
			}
			compiled, err := compilePoint(group, point, typesByKey)
			if err != nil {
				return nil, err
			}
			result.ByKey[compiled.Key] = compiled
		}
	}

	order, err := topologicalOrder(result.ByKey)
	if err != nil {
		return nil, err
	}
	for _, key := range order {
		result.Points = append(result.Points, result.ByKey[key])
	}
	return result, nil
}

func compilePoint(group config.EdgeComputeGroupConfig, point config.EdgeComputedPointConfig, typesByKey map[string]string) (*CompiledPoint, error) {
	key := sourceKey(group.GroupKey, point.Metric)
	if len(point.Expression) == 0 || len(point.Expression) > maxExpressionLength {
		return nil, fmt.Errorf("派生点 %s 的表达式长度必须为 1-%d", key, maxExpressionLength)
	}
	if len(point.Inputs) == 0 || len(point.Inputs) > maxInputsPerPoint {
		return nil, fmt.Errorf("派生点 %s 的输入数量必须为 1-%d", key, maxInputsPerPoint)
	}
	// CEL's built-in modulo operator only accepts integer operands. Edge-compute
	// normalizes all numeric inputs to double, so remove that single overload and
	// install the bounded double implementation below. Macros stay disabled to
	// keep formulas expression-only (no comprehensions or implicit loops).
	standardLibrary := celenv.NewLibrarySubset().
		SetDisableMacros(true).
		AddIncludedFunctions(
			&celenv.Function{Name: operators.Conditional},
			&celenv.Function{Name: operators.LogicalAnd},
			&celenv.Function{Name: operators.LogicalOr},
			&celenv.Function{Name: operators.LogicalNot},
			&celenv.Function{Name: operators.Negate},
		)
	options := []cel.EnvOption{cel.StdLib(cel.StdLibSubset(standardLibrary))}
	// Collected numeric values are deliberately normalized to double, while CEL
	// parses natural constants such as `1` and `100` as int. Without mixed
	// overloads, ordinary industrial formulas such as `temperature + 1` or
	// `clamp(level, 0, 100)` fail type checking and force operators to write
	// surprising `.0` suffixes. Keep the runtime representation as double, but
	// accept either literal form at the expression boundary.
	options = append(options, mixedNumericFunctions()...)
	inputTypes := map[string]string{}
	seenAliases := map[string]bool{}
	for _, input := range point.Inputs {
		if !aliasPattern.MatchString(input.Alias) {
			return nil, fmt.Errorf("派生点 %s 的输入别名 %q 无效", key, input.Alias)
		}
		if seenAliases[input.Alias] {
			return nil, fmt.Errorf("派生点 %s 的输入别名 %q 重复", key, input.Alias)
		}
		seenAliases[input.Alias] = true
		source := sourceKey(input.SourceDeviceKey, input.SourceMetric)
		dataType, exists := typesByKey[source]
		if !exists {
			return nil, fmt.Errorf("派生点 %s 引用了不存在的输入 %s", key, source)
		}
		if dataType != "bool" && !supportedOutputType(dataType) {
			return nil, fmt.Errorf("派生点 %s 的输入 %s 类型 %s 不能参与计算", key, source, dataType)
		}
		inputTypes[input.Alias] = dataType
		if dataType == "bool" {
			options = append(options, cel.Variable(input.Alias, cel.BoolType))
		} else {
			options = append(options, cel.Variable(input.Alias, cel.DoubleType))
		}
	}
	options = append(options, mathFunctions()...)
	env, err := cel.NewCustomEnv(options...)
	if err != nil {
		return nil, fmt.Errorf("派生点 %s 创建表达式环境失败: %w", key, err)
	}
	ast, issues := env.Compile(point.Expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("派生点 %s 表达式错误: %w", key, issues.Err())
	}
	outputType := normalizedDataType(point.DataType)
	if outputType == "bool" && ast.OutputType() != cel.BoolType {
		return nil, fmt.Errorf("派生点 %s 表达式结果必须为 bool，实际为 %s", key, ast.OutputType())
	}
	if outputType != "bool" && ast.OutputType() != cel.DoubleType && ast.OutputType() != cel.IntType && ast.OutputType() != cel.UintType {
		return nil, fmt.Errorf("派生点 %s 表达式结果必须为数值，实际为 %s", key, ast.OutputType())
	}
	program, err := env.Program(ast, cel.CostLimit(maxEvaluationCost))
	if err != nil {
		return nil, fmt.Errorf("派生点 %s 编译失败: %w", key, err)
	}
	signatureRaw, _ := json.Marshal(struct {
		Expression string
		Inputs     []config.EdgeComputeInputConfig
		DataType   string
	}{point.Expression, point.Inputs, point.DataType})
	return &CompiledPoint{Group: group, Config: point, Key: key, Signature: string(signatureRaw), Program: program, InputTypes: inputTypes}, nil
}

func topologicalOrder(points map[string]*CompiledPoint) ([]string, error) {
	state := map[string]uint8{}
	var order []string
	var visit func(string) error
	visit = func(key string) error {
		switch state[key] {
		case 1:
			return fmt.Errorf("边缘计算存在循环依赖，涉及 %s", key)
		case 2:
			return nil
		}
		state[key] = 1
		for _, input := range points[key].Config.Inputs {
			dependency := sourceKey(input.SourceDeviceKey, input.SourceMetric)
			if _, computed := points[dependency]; computed {
				if err := visit(dependency); err != nil {
					return err
				}
			}
		}
		state[key] = 2
		order = append(order, key)
		return nil
	}
	for key := range points {
		if err := visit(key); err != nil {
			return nil, err
		}
	}
	return order, nil
}

func mathFunctions() []cel.EnvOption {
	unary := func(name string, fn func(float64) float64) cel.EnvOption {
		return cel.Function(name, cel.Overload(name+"_double", []*cel.Type{cel.DoubleType}, cel.DoubleType,
			cel.UnaryBinding(func(value ref.Val) ref.Val { return types.Double(fn(float64(value.(types.Double)))) })))
	}
	binary := func(name string, fn func(float64, float64) float64) cel.EnvOption {
		return cel.Function(name, cel.Overload(name+"_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType,
			cel.BinaryBinding(func(left, right ref.Val) ref.Val {
				return types.Double(fn(float64(left.(types.Double)), float64(right.(types.Double))))
			})))
	}
	clamp := cel.Function("clamp", cel.Overload("clamp_double_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType, cel.DoubleType}, cel.DoubleType,
		cel.FunctionBinding(func(values ...ref.Val) ref.Val {
			value, low, high := float64(values[0].(types.Double)), float64(values[1].(types.Double)), float64(values[2].(types.Double))
			if low > high {
				return types.NewErr("clamp lower bound exceeds upper bound")
			}
			return types.Double(math.Min(math.Max(value, low), high))
		})))
	return []cel.EnvOption{
		unary("abs", math.Abs), unary("round", math.Round), unary("floor", math.Floor), unary("ceil", math.Ceil),
		binary("min", math.Min), binary("max", math.Max), clamp,
	}
}

func mixedNumericFunctions() []cel.EnvOption {
	toDouble := func(value ref.Val) (float64, bool) {
		switch typed := value.(type) {
		case types.Double:
			return float64(typed), true
		case types.Int:
			return float64(typed), true
		case types.Uint:
			return float64(typed), true
		default:
			return 0, false
		}
	}
	numericOperator := func(name, id string, fn func(float64, float64) (float64, error)) cel.EnvOption {
		numericTypes := []struct {
			name     string
			typeName *cel.Type
		}{{"double", cel.DoubleType}, {"int", cel.IntType}, {"uint", cel.UintType}}
		overloads := make([]cel.FunctionOpt, 0, len(numericTypes)*len(numericTypes)+1)
		for _, left := range numericTypes {
			for _, right := range numericTypes {
				overloads = append(overloads, cel.Overload(id+"_"+left.name+"_"+right.name, []*cel.Type{left.typeName, right.typeName}, cel.DoubleType))
			}
		}
		overloads = append(overloads, cel.SingletonBinaryBinding(func(left, right ref.Val) ref.Val {
			leftNumber, leftOK := toDouble(left)
			rightNumber, rightOK := toDouble(right)
			if !leftOK || !rightOK {
				return types.NewErr("numeric operands required")
			}
			value, err := fn(leftNumber, rightNumber)
			if err != nil {
				return types.NewErr("%s", err)
			}
			return types.Double(value)
		}))
		return cel.Function(name, overloads...)
	}
	numericComparison := func(name, id string, fn func(float64, float64) bool) cel.EnvOption {
		numericTypes := []struct {
			name     string
			typeName *cel.Type
		}{{"double", cel.DoubleType}, {"int", cel.IntType}, {"uint", cel.UintType}}
		overloads := make([]cel.FunctionOpt, 0, len(numericTypes)*len(numericTypes)+1)
		for _, left := range numericTypes {
			for _, right := range numericTypes {
				overloads = append(overloads, cel.Overload(id+"_"+left.name+"_"+right.name, []*cel.Type{left.typeName, right.typeName}, cel.BoolType))
			}
		}
		overloads = append(overloads, cel.SingletonBinaryBinding(func(left, right ref.Val) ref.Val {
			leftNumber, leftOK := toDouble(left)
			rightNumber, rightOK := toDouble(right)
			if !leftOK || !rightOK {
				return types.NewErr("numeric operands required")
			}
			return types.Bool(fn(leftNumber, rightNumber))
		}))
		return cel.Function(name, overloads...)
	}
	equality := func(name, id string, fn func(bool) bool) cel.EnvOption {
		numericTypes := []struct {
			name     string
			typeName *cel.Type
		}{{"double", cel.DoubleType}, {"int", cel.IntType}, {"uint", cel.UintType}}
		overloads := make([]cel.FunctionOpt, 0, len(numericTypes)*len(numericTypes)+2)
		for _, left := range numericTypes {
			for _, right := range numericTypes {
				overloads = append(overloads, cel.Overload(id+"_"+left.name+"_"+right.name, []*cel.Type{left.typeName, right.typeName}, cel.BoolType))
			}
		}
		overloads = append(overloads, cel.Overload(id+"_bool_bool", []*cel.Type{cel.BoolType, cel.BoolType}, cel.BoolType))
		overloads = append(overloads, cel.SingletonBinaryBinding(func(left, right ref.Val) ref.Val {
			leftBool, leftIsBool := left.(types.Bool)
			rightBool, rightIsBool := right.(types.Bool)
			if leftIsBool || rightIsBool {
				if !leftIsBool || !rightIsBool {
					return types.NewErr("operands must have compatible types")
				}
				return types.Bool(fn(bool(leftBool) == bool(rightBool)))
			}
			leftNumber, leftOK := toDouble(left)
			rightNumber, rightOK := toDouble(right)
			if !leftOK || !rightOK {
				return types.NewErr("numeric or bool operands required")
			}
			return types.Bool(fn(leftNumber == rightNumber))
		}))
		return cel.Function(name, overloads...)
	}
	binary := func(name, id string, leftType, rightType *cel.Type, fn func(float64, float64) float64) cel.EnvOption {
		return cel.Function(name, cel.Overload(id, []*cel.Type{leftType, rightType}, cel.DoubleType,
			cel.BinaryBinding(func(left, right ref.Val) ref.Val {
				leftNumber, leftOK := toDouble(left)
				rightNumber, rightOK := toDouble(right)
				if !leftOK || !rightOK {
					return types.NewErr("numeric operands required")
				}
				return types.Double(fn(leftNumber, rightNumber))
			})))
	}
	options := []cel.EnvOption{
		numericOperator(operators.Add, "edge_add", func(left, right float64) (float64, error) { return left + right, nil }),
		numericOperator(operators.Subtract, "edge_subtract", func(left, right float64) (float64, error) { return left - right, nil }),
		numericOperator(operators.Multiply, "edge_multiply", func(left, right float64) (float64, error) { return left * right, nil }),
		numericOperator(operators.Divide, "edge_divide", func(left, right float64) (float64, error) {
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		}),
		numericOperator(operators.Modulo, "edge_modulo", func(left, right float64) (float64, error) {
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			return math.Mod(left, right), nil
		}),
		numericComparison(operators.Less, "edge_less", func(left, right float64) bool { return left < right }),
		numericComparison(operators.LessEquals, "edge_less_equals", func(left, right float64) bool { return left <= right }),
		numericComparison(operators.Greater, "edge_greater", func(left, right float64) bool { return left > right }),
		numericComparison(operators.GreaterEquals, "edge_greater_equals", func(left, right float64) bool { return left >= right }),
		equality(operators.Equals, "edge_equals", func(equal bool) bool { return equal }),
		equality(operators.NotEquals, "edge_not_equals", func(equal bool) bool { return !equal }),
	}
	options = append(options,
		binary("min", "min_double_int", cel.DoubleType, cel.IntType, math.Min),
		binary("min", "min_int_double", cel.IntType, cel.DoubleType, math.Min),
		binary("max", "max_double_int", cel.DoubleType, cel.IntType, math.Max),
		binary("max", "max_int_double", cel.IntType, cel.DoubleType, math.Max),
	)

	for mask := 1; mask < 8; mask++ {
		argumentTypes := make([]*cel.Type, 3)
		typeNames := make([]string, 3)
		for index := range argumentTypes {
			if mask&(1<<index) != 0 {
				argumentTypes[index] = cel.IntType
				typeNames[index] = "int"
			} else {
				argumentTypes[index] = cel.DoubleType
				typeNames[index] = "double"
			}
		}
		id := "clamp_" + strings.Join(typeNames, "_")
		options = append(options, cel.Function("clamp", cel.Overload(id, argumentTypes, cel.DoubleType,
			cel.FunctionBinding(func(values ...ref.Val) ref.Val {
				value, valueOK := toDouble(values[0])
				low, lowOK := toDouble(values[1])
				high, highOK := toDouble(values[2])
				if !valueOK || !lowOK || !highOK {
					return types.NewErr("numeric operands required")
				}
				if low > high {
					return types.NewErr("clamp lower bound exceeds upper bound")
				}
				return types.Double(math.Min(math.Max(value, low), high))
			}))))
	}
	return options
}

func NormalizeInput(dataType string, value interface{}) (interface{}, error) {
	if normalizedDataType(dataType) == "bool" {
		if typed, ok := value.(bool); ok {
			return typed, nil
		}
		if number, ok := numberAsFloat64(value); ok && !math.IsNaN(number) && !math.IsInf(number, 0) {
			return number != 0, nil
		}
		return nil, fmt.Errorf("值 %v 不能转换为 bool", value)
	}
	number, ok := numberAsFloat64(value)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
		return nil, fmt.Errorf("值 %v 不是有效数值", value)
	}
	return number, nil
}

func ConvertOutput(dataType string, value interface{}) (interface{}, error) {
	typeName := normalizedDataType(dataType)
	if typeName == "bool" {
		if typed, ok := value.(bool); ok {
			return typed, nil
		}
		return nil, fmt.Errorf("结果 %v 不是 bool", value)
	}
	number, ok := numberAsFloat64(value)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
		return nil, fmt.Errorf("结果 %v 不是有效数值", value)
	}
	switch typeName {
	case "float32":
		if math.Abs(number) > math.MaxFloat32 {
			return nil, fmt.Errorf("结果 %v 超出 float32 范围", number)
		}
		return float64(float32(number)), nil
	case "float64":
		return number, nil
	case "int16", "uint16", "int32", "uint32":
		if math.Trunc(number) != number {
			return nil, fmt.Errorf("整数输出不能包含小数: %v", number)
		}
	}
	switch typeName {
	case "int16":
		if number < math.MinInt16 || number > math.MaxInt16 {
			return nil, fmt.Errorf("结果 %v 超出 int16 范围", number)
		}
		return int16(number), nil
	case "uint16":
		if number < 0 || number > math.MaxUint16 {
			return nil, fmt.Errorf("结果 %v 超出 uint16 范围", number)
		}
		return uint16(number), nil
	case "int32":
		if number < math.MinInt32 || number > math.MaxInt32 {
			return nil, fmt.Errorf("结果 %v 超出 int32 范围", number)
		}
		return int32(number), nil
	case "uint32":
		if number < 0 || number > math.MaxUint32 {
			return nil, fmt.Errorf("结果 %v 超出 uint32 范围", number)
		}
		return uint32(number), nil
	default:
		return nil, fmt.Errorf("不支持的输出类型 %s", dataType)
	}
}

func numberAsFloat64(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case types.Double:
		return float64(typed), true
	case types.Int:
		return float64(typed), true
	case types.Uint:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func normalizedDataType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "float64"
	}
	return value
}

func supportedOutputType(value string) bool {
	switch value {
	case "bool", "int16", "uint16", "int32", "uint32", "float32", "float64":
		return true
	default:
		return false
	}
}

func sourceKey(deviceKey, metric string) string { return deviceKey + "::" + metric }
