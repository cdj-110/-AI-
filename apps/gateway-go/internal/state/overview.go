package state

import (
	"sort"
	"strings"
	"time"
)

type DeviceOverview struct {
	DeviceKey string `json:"deviceKey"`
	Total     int    `json:"total"`
	Healthy   int    `json:"healthy"`
	Failed    int    `json:"failed"`
	Pending   int    `json:"pending"`
	Stale     int    `json:"stale"`
}
type IssueOverview struct {
	DeviceKey string `json:"deviceKey"`
	Reason    string `json:"reason"`
	Detail    string `json:"detail"`
	Count     int    `json:"count"`
	Protocol  string `json:"protocol"`
	Address   string `json:"address"`
}
type CollectionOverview struct {
	Devices        []DeviceOverview `json:"devices"`
	Issues         []IssueOverview  `json:"issues"`
	IssueGroups    int              `json:"issueGroups"`
	AffectedPoints int              `json:"affectedPoints"`
}

func issueReason(p PointStatus, stale bool) string {
	text := strings.ToLower(p.Error)
	switch {
	case strings.Contains(text, "deadline exceeded"), strings.Contains(text, "timeout"), strings.Contains(text, "超时"):
		return "连接或读取超时"
	case strings.Contains(text, "connection refused"):
		return "设备拒绝连接"
	case strings.Contains(text, "no route"), strings.Contains(text, "unreachable"):
		return "设备网络不可达"
	case strings.Contains(text, "数据标识"), strings.Contains(text, "checksum"), strings.Contains(text, "校验"):
		return "响应报文或数据标识异常"
	case p.Error != "":
		return "采集失败"
	case stale:
		return "数据长时间未更新"
	default:
		return "数据质量异常"
	}
}

// CollectionOverview transfers device summaries and at most five grouped issues,
// never the full point list or its values (including for large projects).
func (s *Store) CollectionOverview() CollectionOverview {
	result := CollectionOverview{Devices: []DeviceOverview{}, Issues: []IssueOverview{}}
	devices := map[string]*DeviceOverview{}
	issues := map[string]*IssueOverview{}
	now := time.Now()
	s.mu.RLock()
	for _, key := range s.pointOrder {
		p, ok := s.points[key]
		if !ok {
			continue
		}
		d := devices[p.DeviceKey]
		if d == nil {
			d = &DeviceOverview{DeviceKey: p.DeviceKey}
			devices[p.DeviceKey] = d
		}
		d.Total++
		stale := pointIsStale(p, now)
		bad := p.Error != "" || string(p.Quality) == "BAD" || string(p.Quality) == "OFFLINE" || string(p.Quality) == "UNCERTAIN"
		switch {
		case bad:
			d.Failed++
		case stale:
			d.Stale++
		case p.UpdatedAt == nil:
			d.Pending++
		default:
			d.Healthy++
		}
		if !bad && !stale {
			continue
		}
		result.AffectedPoints++
		reason := issueReason(p, stale)
		groupKey := p.DeviceKey + "\x00" + reason
		issue := issues[groupKey]
		if issue == nil {
			detail := p.Error
			if detail == "" {
				detail = p.Diagnostic
			}
			if detail == "" {
				detail = reason
			}
			if runes := []rune(detail); len(runes) > 600 {
				detail = string(runes[:600]) + "…"
			}
			issue = &IssueOverview{DeviceKey: p.DeviceKey, Reason: reason, Detail: detail, Protocol: p.Protocol, Address: p.Address}
			issues[groupKey] = issue
		}
		issue.Count++
	}
	s.mu.RUnlock()
	for _, d := range devices {
		result.Devices = append(result.Devices, *d)
	}
	sort.Slice(result.Devices, func(i, j int) bool { return result.Devices[i].DeviceKey < result.Devices[j].DeviceKey })
	keys := make([]string, 0, len(issues))
	for k := range issues {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result.IssueGroups = len(keys)
	for _, k := range keys {
		if len(result.Issues) == 5 {
			break
		}
		result.Issues = append(result.Issues, *issues[k])
	}
	return result
}
