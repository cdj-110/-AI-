package state

import (
	"fmt"
	"testing"
	"time"
	"weikong-iot-platform/apps/gateway-go/internal/model"
)

func TestCollectionOverviewBoundedAndStable(t *testing.T) {
	now := time.Now()
	s := &Store{points: map[string]PointStatus{}}
	for i := 0; i < 20; i++ {
		key := fmt.Sprint(i)
		s.pointOrder = append(s.pointOrder, key)
		s.points[key] = PointStatus{DeviceKey: fmt.Sprintf("d%02d", i/2), Metric: key, Error: "context deadline exceeded", Value: "must not be exported"}
	}
	a, b := s.CollectionOverview(), s.CollectionOverview()
	if len(a.Issues) != 5 || a.IssueGroups != 10 || a.AffectedPoints != 20 || len(a.Devices) != 10 {
		t.Fatalf("unexpected summary: %+v", a)
	}
	for i, g := range a.Issues {
		if g.Count != 2 || g.DeviceKey != b.Issues[i].DeviceKey {
			t.Fatal("unstable grouping")
		}
	}
	s.points["0"] = PointStatus{DeviceKey: "d00", UpdatedAt: &now, Quality: model.PointQuality("GOOD")}
	s.points["1"] = PointStatus{DeviceKey: "d00", Quality: model.PointQuality("BAD")}
	a = s.CollectionOverview()
	if a.Devices[0].Healthy != 1 || a.Devices[0].Failed != 1 {
		t.Fatalf("mixed quality: %+v", a.Devices[0])
	}
	s.points["1"] = PointStatus{DeviceKey: "d00", UpdatedAt: &now}
	a = s.CollectionOverview()
	if a.Devices[0].Failed != 0 || a.AffectedPoints != 18 {
		t.Fatal("recovery not reflected")
	}
}
func TestCollectionOverviewPendingAndStale(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	s := &Store{pointOrder: []string{"a", "b"}, points: map[string]PointStatus{"a": {DeviceKey: "a"}, "b": {DeviceKey: "b", UpdatedAt: &old, StaleAfterSeconds: 10}}}
	a := s.CollectionOverview()
	if a.Devices[0].Pending != 1 || a.Devices[1].Stale != 1 || a.AffectedPoints != 1 {
		t.Fatalf("unexpected: %+v", a)
	}
}
