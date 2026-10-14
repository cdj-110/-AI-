package web

import (
	"encoding/json"
	"testing"

	"weikong-iot-platform/apps/gateway-go/internal/config"
	"weikong-iot-platform/apps/gateway-go/internal/history"
)

func TestHistoryVolumeBindingSurvivesLegacyForm(t *testing.T) {
	previous := config.HistoryStorageConfig{StoragePath: "/media/udisk1", StorageUUID: "original-volume"}
	next := config.HistoryStorageConfig{StoragePath: previous.StoragePath}
	bindHistoryVolume(&next, previous)
	if next.StorageUUID != previous.StorageUUID {
		t.Fatal("old UI discarded binding")
	}
	data, err := json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	var decoded config.HistoryStorageConfig
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.StorageUUID != previous.StorageUUID {
		t.Fatalf("binding lost in JSON: %v", err)
	}
	next.StoragePath = config.DefaultHistoryStoragePath
	bindHistoryVolume(&next, previous)
	if next.StorageUUID != "" {
		t.Fatal("local storage retained removable binding")
	}
}

func TestResolveHistoryStorageDefaultsToLocalWithoutRemovableMedia(t *testing.T) {
	historyConfig := config.HistoryStorageConfig{}
	historyConfig.ApplyDefaults()
	path, storageType, available, ok := resolveHistoryStorage(historyConfig)
	if !ok || storageType != "local" || available != nil {
		t.Fatalf("local history location was not resolved: path=%q type=%q ok=%v", path, storageType, ok)
	}
	if want := history.Path(config.DefaultHistoryStoragePath); path != want {
		t.Fatalf("local history path = %q, want %q", path, want)
	}
}

func TestResolveHistoryStorageDoesNotTreatMissingUSBAsLocal(t *testing.T) {
	historyConfig := config.HistoryStorageConfig{StoragePath: "/media/definitely-not-mounted"}
	if path, storageType, _, ok := resolveHistoryStorage(historyConfig); ok || path != "" || storageType != "removable" {
		t.Fatalf("missing removable storage was accepted: path=%q type=%q ok=%v", path, storageType, ok)
	}
}
