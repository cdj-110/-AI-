package collector

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

type OPCUA struct{}

type OPCUAConnectionResult struct {
	Endpoint string
}

type OPCUABrowseNode struct {
	NodeID      string `json:"nodeId"`
	BrowseName  string `json:"browseName"`
	DisplayName string `json:"displayName"`
	NodeClass   string `json:"nodeClass"`
	DataType    string `json:"dataType,omitempty"`
	HasChildren bool   `json:"hasChildren"`
	Selectable  bool   `json:"selectable"`
}

type opcuaConnection struct {
	client *opcua.Client
	mu     sync.Mutex
}

var opcuaPool = struct {
	sync.Mutex
	items map[string]*opcuaConnection
}{items: map[string]*opcuaConnection{}}

func (OPCUA) ReadPoint(ctx context.Context, point config.PointConfig) (result model.PointValue, resultErr error) {
	nodeID := strings.TrimSpace(point.NodeID)
	recordSemanticRequest("opcua", point, "ReadValue 请求 NodeId="+nodeID)
	defer func() {
		if resultErr != nil {
			recordSemanticResponse("opcua", point, "ReadValue 响应失败 NodeId="+nodeID, resultErr)
		} else {
			recordSemanticResponse("opcua", point, fmt.Sprintf("ReadValue 响应 NodeId=%s 值=%v", nodeID, result.Value), nil)
		}
	}()
	if nodeID == "" {
		return model.PointValue{}, fmt.Errorf("OPC UA NodeId 不能为空，例如 ns=2;s=Temperature")
	}
	id, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return model.PointValue{}, fmt.Errorf("OPC UA NodeId %q 无效：%w", nodeID, err)
	}

	conn, err := getOPCUAConnection(ctx, point)
	if err != nil {
		return model.PointValue{}, err
	}
	value, err := readOPCUANode(ctx, conn, id)
	if err != nil {
		closeOPCUAConnection(ctx, point, conn)
		if ctx.Err() != nil {
			return model.PointValue{}, ctx.Err()
		}
		conn, reconnectErr := getOPCUAConnection(ctx, point)
		if reconnectErr != nil {
			return model.PointValue{}, fmt.Errorf("OPC UA 读取失败且重连失败（首次错误：%v）：%w", err, reconnectErr)
		}
		value, err = readOPCUANode(ctx, conn, id)
		if err != nil {
			closeOPCUAConnection(ctx, point, conn)
			return model.PointValue{}, fmt.Errorf("OPC UA 重连后读取 NodeId %s 失败：%w", nodeID, err)
		}
	}
	return model.PointValue{DeviceKey: point.DeviceKey, Metric: point.Metric, Value: applyOPCUAScale(point, value)}, nil
}

func TestOPCUAConnection(ctx context.Context, point config.PointConfig) (OPCUAConnectionResult, error) {
	endpoint, err := opcuaEndpoint(point.Address)
	result := OPCUAConnectionResult{Endpoint: endpoint}
	if err != nil {
		return result, err
	}
	client, err := newOPCUAClient(ctx, endpoint, point.Username, point.Password)
	if err != nil {
		return result, err
	}
	if err := client.Connect(ctx); err != nil {
		return result, fmt.Errorf("OPC UA 连接失败 %s：%w", endpoint, err)
	}
	_ = client.Close(ctx)
	return result, nil
}

func BrowseOPCUANodes(ctx context.Context, point config.PointConfig, parentNodeID string) ([]OPCUABrowseNode, error) {
	if strings.TrimSpace(parentNodeID) == "" {
		parentNodeID = "i=84"
	}
	parentID, err := ua.ParseNodeID(parentNodeID)
	if err != nil {
		return nil, fmt.Errorf("OPC UA 父节点 NodeId %q 无效：%w", parentNodeID, err)
	}
	conn, err := getOPCUAConnection(ctx, point)
	if err != nil {
		return nil, err
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	references, err := conn.client.Node(parentID).References(ctx, id.HierarchicalReferences, ua.BrowseDirectionForward, ua.NodeClassAll, true)
	if err != nil {
		return nil, fmt.Errorf("浏览 OPC UA 节点 %s 失败：%w", parentNodeID, err)
	}
	nodes := make([]OPCUABrowseNode, 0, len(references))
	for _, reference := range references {
		if reference == nil || reference.NodeID == nil || reference.NodeID.NodeID == nil {
			continue
		}
		node := OPCUABrowseNode{
			NodeID:      reference.NodeID.NodeID.String(),
			NodeClass:   reference.NodeClass.String(),
			HasChildren: reference.NodeClass != ua.NodeClassVariable,
			Selectable:  reference.NodeClass == ua.NodeClassVariable,
		}
		if reference.BrowseName != nil {
			node.BrowseName = reference.BrowseName.Name
		}
		if reference.DisplayName != nil {
			node.DisplayName = reference.DisplayName.Text
		}
		if node.DisplayName == "" {
			node.DisplayName = node.BrowseName
		}
		if node.Selectable {
			attrs, attrErr := conn.client.Node(reference.NodeID.NodeID).Attributes(ctx, ua.AttributeIDDataType)
			if attrErr == nil && len(attrs) > 0 && attrs[0].Status == ua.StatusOK && attrs[0].Value != nil {
				node.DataType = opcuaDataTypeName(attrs[0].Value.NodeID())
			}
			if node.DataType == "" {
				node.DataType = "auto"
			}
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func BrowseOPCUAVariableNodes(ctx context.Context, point config.PointConfig, parentNodeID string, limit int) ([]OPCUABrowseNode, error) {
	if limit <= 0 {
		limit = 1000
	}
	var variables []OPCUABrowseNode
	seen := map[string]bool{}
	queue := []string{strings.TrimSpace(parentNodeID)}
	if queue[0] == "" {
		queue[0] = "i=84"
	}
	for len(queue) > 0 {
		if ctx.Err() != nil {
			return variables, ctx.Err()
		}
		nodeID := queue[0]
		queue = queue[1:]
		if seen[nodeID] {
			continue
		}
		seen[nodeID] = true
		children, err := BrowseOPCUANodes(ctx, point, nodeID)
		if err != nil {
			return variables, err
		}
		for _, child := range children {
			if child.NodeID == "" || seen[child.NodeID] {
				continue
			}
			if child.Selectable {
				variables = append(variables, child)
				if len(variables) >= limit {
					return variables, nil
				}
				continue
			}
			if child.HasChildren {
				queue = append(queue, child.NodeID)
			}
		}
	}
	return variables, nil
}

func opcuaDataTypeName(nodeID *ua.NodeID) string {
	if nodeID == nil || nodeID.Namespace() != 0 {
		return "auto"
	}
	switch nodeID.IntID() {
	case id.Boolean:
		return "bool"
	case id.SByte:
		return "int8"
	case id.Byte:
		return "uint8"
	case id.Int16:
		return "int16"
	case id.UInt16:
		return "uint16"
	case id.Int32:
		return "int32"
	case id.UInt32:
		return "uint32"
	case id.Int64:
		return "int64"
	case id.UInt64:
		return "uint64"
	case id.Float:
		return "float32"
	case id.Double:
		return "float64"
	case id.String:
		return "string"
	case id.DateTime, id.UtcTime:
		return "datetime"
	default:
		return "auto"
	}
}

func getOPCUAConnection(ctx context.Context, point config.PointConfig) (*opcuaConnection, error) {
	endpoint, err := opcuaEndpoint(point.Address)
	if err != nil {
		return nil, err
	}
	key := opcuaConnectionKey(endpoint, point.Username, point.Password)
	opcuaPool.Lock()
	if conn := opcuaPool.items[key]; conn != nil {
		opcuaPool.Unlock()
		return conn, nil
	}
	opcuaPool.Unlock()

	client, err := newOPCUAClient(ctx, endpoint, point.Username, point.Password)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("OPC UA 连接失败 %s：%w", endpoint, err)
	}
	conn := &opcuaConnection{client: client}
	opcuaPool.Lock()
	if existing := opcuaPool.items[key]; existing != nil {
		opcuaPool.Unlock()
		_ = client.Close(ctx)
		return existing, nil
	}
	opcuaPool.items[key] = conn
	opcuaPool.Unlock()
	return conn, nil
}

func newOPCUAClient(ctx context.Context, endpoint, username, password string) (*opcua.Client, error) {
	authType := ua.UserTokenTypeAnonymous
	if strings.TrimSpace(username) != "" {
		authType = ua.UserTokenTypeUserName
	}
	endpoints, err := opcua.GetEndpoints(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("读取 OPC UA Endpoint 描述失败 %s：%w", endpoint, err)
	}
	selected := selectOPCUAEndpoint(endpoints, authType)
	if selected == nil {
		return nil, fmt.Errorf("OPC UA 服务器未提供支持 %s 认证的 None 安全端点", opcuaAuthLabel(authType))
	}
	options := []opcua.Option{
		opcua.SecurityFromEndpoint(selected, authType),
		opcua.RequestTimeout(5 * time.Second),
		opcua.DialTimeout(5 * time.Second),
		opcua.AutoReconnect(true),
	}
	if strings.TrimSpace(username) == "" {
		options = append(options, opcua.AuthAnonymous())
	} else {
		options = append(options, opcua.AuthUsername(username, password))
	}
	client, err := opcua.NewClient(endpoint, options...)
	if err != nil {
		return nil, fmt.Errorf("创建 OPC UA 客户端失败：%w", err)
	}
	return client, nil
}

func selectOPCUAEndpoint(endpoints []*ua.EndpointDescription, authType ua.UserTokenType) *ua.EndpointDescription {
	for _, endpoint := range endpoints {
		if endpoint.SecurityMode != ua.MessageSecurityModeNone || endpoint.SecurityPolicyURI != ua.SecurityPolicyURINone {
			continue
		}
		for _, token := range endpoint.UserIdentityTokens {
			if token.TokenType == authType {
				return endpoint
			}
		}
	}
	return nil
}

func opcuaAuthLabel(authType ua.UserTokenType) string {
	if authType == ua.UserTokenTypeUserName {
		return "用户名/密码"
	}
	return "匿名"
}

func readOPCUANode(ctx context.Context, conn *opcuaConnection, id *ua.NodeID) (interface{}, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	response, err := conn.client.Read(ctx, &ua.ReadRequest{
		MaxAge:             0,
		TimestampsToReturn: ua.TimestampsToReturnBoth,
		NodesToRead: []*ua.ReadValueID{{
			NodeID:      id,
			AttributeID: ua.AttributeIDValue,
		}},
	})
	if err != nil {
		return nil, err
	}
	if response == nil || len(response.Results) == 0 {
		return nil, fmt.Errorf("服务器未返回 NodeId %s 的读取结果", id)
	}
	result := response.Results[0]
	if result.Status != ua.StatusOK {
		return nil, fmt.Errorf("NodeId %s 返回状态 %s", id, result.Status)
	}
	if result.Value == nil {
		return nil, fmt.Errorf("NodeId %s 返回空值", id)
	}
	return result.Value.Value(), nil
}

func closeOPCUAConnection(ctx context.Context, point config.PointConfig, target *opcuaConnection) {
	endpoint, err := opcuaEndpoint(point.Address)
	if err != nil {
		return
	}
	key := opcuaConnectionKey(endpoint, point.Username, point.Password)
	opcuaPool.Lock()
	conn := opcuaPool.items[key]
	if conn == target {
		delete(opcuaPool.items, key)
	}
	opcuaPool.Unlock()
	if conn == target {
		conn.mu.Lock()
		_ = conn.client.Close(ctx)
		conn.mu.Unlock()
	}
}

func opcuaConnectionKey(endpoint, username, password string) string {
	credentialHash := sha256.Sum256([]byte(username + "\x00" + password))
	return fmt.Sprintf("%s#%x", endpoint, credentialHash)
}

func opcuaEndpoint(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "opc.tcp://127.0.0.1:4840", nil
	}
	if !strings.Contains(address, "://") {
		address = "opc.tcp://" + address
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "opc.tcp" || parsed.Hostname() == "" {
		return address, fmt.Errorf("OPC UA Endpoint 无效：%s", address)
	}
	if parsed.Port() == "" {
		parsed.Host = net.JoinHostPort(parsed.Hostname(), strconv.Itoa(4840))
	}
	return parsed.String(), nil
}

func applyOPCUAScale(point config.PointConfig, value interface{}) interface{} {
	if point.Scale == 1 && point.Offset == 0 {
		return value
	}
	var number float64
	switch typed := value.(type) {
	case int:
		number = float64(typed)
	case int8:
		number = float64(typed)
	case int16:
		number = float64(typed)
	case int32:
		number = float64(typed)
	case int64:
		number = float64(typed)
	case uint:
		number = float64(typed)
	case uint8:
		number = float64(typed)
	case uint16:
		number = float64(typed)
	case uint32:
		number = float64(typed)
	case uint64:
		number = float64(typed)
	case float32:
		number = float64(typed)
	case float64:
		number = typed
	default:
		return value
	}
	return number*point.Scale + point.Offset
}
