package collector

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

// OPCDA is a thin gateway adapter. COM, DCOM, RPC, HRESULT and VARIANT stay in
// the native runtime and are never exposed to application packages.
type OPCDA struct{}

type OPCDAConnectionResult struct {
	Address string
	Stage   string
}

// OPCDABrowseNode is one namespace entry returned by
// IOPCBrowseServerAddressSpace. Branches can be expanded lazily; leaves are
// directly importable as gateway points.
type OPCDABrowseNode struct {
	Name         string `json:"name"`
	ItemID       string `json:"itemId"`
	DataType     string `json:"dataType,omitempty"`
	Access       string `json:"access,omitempty"`
	AccessRights uint32 `json:"accessRights,omitempty"`
	IsBranch     bool   `json:"isBranch"`
	HasChildren  bool   `json:"hasChildren"`
	Selectable   bool   `json:"selectable"`
}

type opcdaBrowseSessionContextKey struct{}

var opcdaBrowseSessionSequence atomic.Uint64

func withOPCDABrowseSession(ctx context.Context) (context.Context, string) {
	id := strconv.FormatUint(opcdaBrowseSessionSequence.Add(1), 10)
	return context.WithValue(ctx, opcdaBrowseSessionContextKey{}, id), id
}

func opcdaBrowseSessionID(ctx context.Context) string {
	id, _ := ctx.Value(opcdaBrowseSessionContextKey{}).(string)
	return id
}

func (OPCDA) BatchKey(point config.PointConfig) string {
	credential := sha256.Sum256([]byte(point.Domain + "\x00" + point.Username + "\x00" + point.Password + "\x00" + point.Workstation))
	autoReconnect := point.AutoReconnect == nil || *point.AutoReconnect
	return fmt.Sprintf("%s#%s#%s#%s#%s#%s#%s#%s#%d#%d#%s#%d#%g#%d#%s#%t#%x",
		strings.ToLower(strings.TrimSpace(point.DeviceKey)),
		strings.ToLower(strings.TrimSpace(point.Address)),
		strings.ToLower(strings.TrimSpace(point.CLSID)),
		strings.ToLower(strings.TrimSpace(point.ProgID)),
		strings.ToLower(strings.TrimSpace(point.SecurityMode)),
		strings.ToLower(strings.TrimSpace(point.Authentication)),
		strings.ToLower(strings.TrimSpace(point.AuthenticationLevel)),
		strings.TrimSpace(point.GroupName),
		point.ConnectTimeoutMilliseconds,
		point.RequestTimeoutMilliseconds,
		strings.TrimSpace(point.LocalBindIP),
		point.RequestedUpdateRateMS,
		point.Deadband,
		point.LocaleID,
		strings.ToLower(strings.TrimSpace(point.ReadSource)),
		autoReconnect,
		credential,
	)
}

// OPCDASessionLayoutsEqual compares the native connection and item layout
// without the collection policy. Code deciding which device to hot-rebuild
// must use OPCDASessionResetDevices, which includes ReadMode and therefore
// creates a fresh DCOM activation and callback exporter for mode changes.
func OPCDASessionLayoutsEqual(previous, next []config.PointConfig) bool {
	return reflect.DeepEqual(opcdaSessionLayouts(previous), opcdaSessionLayouts(next))
}

// OPCDASessionResetDevices returns the OPC DA devices whose native session
// must be rebuilt during a hot configuration reload. Unlike
// OPCDASessionLayoutsEqual, ReadMode is included deliberately: changing
// between IOPCSyncIO and IOPCDataCallback must use a fresh DCOM activation and
// callback exporter identity, but it must not restart the gateway process or
// disturb unrelated protocol sessions.
func OPCDASessionResetDevices(previous, next []config.PointConfig) []string {
	previousLayouts := opcdaDeviceSessionLayouts(previous, true)
	nextLayouts := opcdaDeviceSessionLayouts(next, true)
	deviceKeys := make(map[string]struct{}, len(previousLayouts)+len(nextLayouts))
	for deviceKey := range previousLayouts {
		deviceKeys[deviceKey] = struct{}{}
	}
	for deviceKey := range nextLayouts {
		deviceKeys[deviceKey] = struct{}{}
	}
	changed := make([]string, 0, len(deviceKeys))
	for deviceKey := range deviceKeys {
		if !reflect.DeepEqual(previousLayouts[deviceKey], nextLayouts[deviceKey]) {
			changed = append(changed, deviceKey)
		}
	}
	sort.Strings(changed)
	return changed
}

func opcdaSessionLayouts(points []config.PointConfig) map[string][]string {
	layouts := map[string][]string{}
	for _, point := range points {
		if config.NormalizeProtocol(point.Protocol) != "opcda" {
			continue
		}
		active := point.Active == nil || *point.Active
		key := (OPCDA{}).BatchKey(point)
		layouts[key] = append(layouts[key], fmt.Sprintf("%s\x00%s\x00%s\x00%t",
			point.ItemID, point.AccessPath,
			strings.ToLower(strings.TrimSpace(point.RequestedDataType)), active))
	}
	for key := range layouts {
		sort.Strings(layouts[key])
	}
	return layouts
}

func opcdaDeviceSessionLayouts(points []config.PointConfig, includeReadMode bool) map[string]map[string][]string {
	layouts := map[string]map[string][]string{}
	for _, point := range points {
		if config.NormalizeProtocol(point.Protocol) != "opcda" {
			continue
		}
		deviceKey := strings.ToLower(strings.TrimSpace(point.DeviceKey))
		if layouts[deviceKey] == nil {
			layouts[deviceKey] = map[string][]string{}
		}
		active := point.Active == nil || *point.Active
		item := fmt.Sprintf("%s\x00%s\x00%s\x00%t",
			point.ItemID, point.AccessPath,
			strings.ToLower(strings.TrimSpace(point.RequestedDataType)), active)
		if includeReadMode {
			item += "\x00" + strings.ToLower(strings.TrimSpace(point.ReadMode))
		}
		key := (OPCDA{}).BatchKey(point)
		layouts[deviceKey][key] = append(layouts[deviceKey][key], item)
	}
	for _, sessions := range layouts {
		for key := range sessions {
			sort.Strings(sessions[key])
		}
	}
	return layouts
}

func (o OPCDA) ReadPoint(ctx context.Context, point config.PointConfig) (model.PointValue, error) {
	results := o.ReadPoints(ctx, []config.PointConfig{point})
	if len(results) == 0 {
		return model.PointValue{}, fmt.Errorf("OPC DA native runtime returned no result")
	}
	return results[0].Value, results[0].Err
}

func (OPCDA) WritePoint(ctx context.Context, point config.PointConfig, value interface{}) error {
	return writeOPCDAPoint(ctx, point, value)
}

func (OPCDA) ReadPoints(ctx context.Context, points []config.PointConfig) []PointReadResult {
	if len(points) == 0 {
		return nil
	}
	return readOPCDAPoints(ctx, points)
}

func isOPCDAVariantDecodeError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "decode IOPCSyncIO::Read response failed")
}

func TestOPCDAConnection(ctx context.Context, point config.PointConfig) (OPCDAConnectionResult, error) {
	result := OPCDAConnectionResult{Address: strings.TrimSpace(point.Address), Stage: "DCOM activation"}
	err := testOPCDAConnection(ctx, point)
	if err != nil {
		return result, err
	}
	result.Stage = "IOPCServer"
	return result, nil
}

// BrowseOPCDANodes returns the direct children under branchItemID.
func BrowseOPCDANodes(ctx context.Context, point config.PointConfig, branchItemID string, limit int) ([]OPCDABrowseNode, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	browseCtx, sessionID := withOPCDABrowseSession(ctx)
	defer closeOPCDABrowseSession(sessionID)
	return browseOPCDANodes(browseCtx, point, branchItemID, limit)
}

// BrowseOPCDALeaves walks the namespace and returns importable leaves only.
// The total result size is bounded so a large server cannot exhaust EG100.
func BrowseOPCDALeaves(ctx context.Context, point config.PointConfig, branchItemID string, limit int) ([]OPCDABrowseNode, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	browseCtx, sessionID := withOPCDABrowseSession(ctx)
	defer closeOPCDABrowseSession(sessionID)
	queue := []string{branchItemID}
	visited := map[string]struct{}{}
	result := make([]OPCDABrowseNode, 0, min(limit, 128))
	for len(queue) > 0 && len(result) < limit {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		branch := queue[0]
		queue = queue[1:]
		key := strings.ToLower(strings.TrimSpace(branch))
		if _, ok := visited[key]; ok {
			continue
		}
		visited[key] = struct{}{}
		nodes, err := browseOPCDANodes(browseCtx, point, branch, limit-len(result))
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			if node.IsBranch {
				if len(visited) < 10000 {
					queue = append(queue, node.ItemID)
				}
				continue
			}
			result = append(result, node)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// CloseOPCDAConnections releases groups, items, remote COM references and RPC
// sessions before a configuration reload.
func CloseOPCDAConnections() {
	closeOPCDASessions()
}

// CloseOPCDAConnectionsForDevices releases only the selected devices. The
// next collection cycle performs RemoteActivation, AddGroup/AddItems and, for
// subscription mode, Advise again inside the existing gateway process.
func CloseOPCDAConnectionsForDevices(deviceKeys []string) {
	closeOPCDASessionsForDevices(deviceKeys)
}

func opcdaSubscriptionRetryDelay(failures int) time.Duration {
	steps := [...]time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, time.Minute}
	if failures <= 1 {
		return steps[0]
	}
	if failures >= len(steps) {
		return steps[len(steps)-1]
	}
	return steps[failures-1]
}
