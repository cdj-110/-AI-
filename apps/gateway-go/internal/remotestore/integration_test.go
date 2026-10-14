package remotestore

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"weikong-iot-platform/apps/gateway-go/internal/config"
)

// Opt-in only. Use a disposable DB: this test creates and removes its own
// randomly named table, never an application table. No default credentials.
func TestRemoteDatabaseIntegration(t *testing.T) {
	driver := os.Getenv("REMOTE_TEST_DRIVER")
	if driver == "" {
		t.Skip("requires disposable database via REMOTE_TEST_* variables")
	}
	port, _ := strconv.Atoi(os.Getenv("REMOTE_TEST_PORT"))
	target := config.RemoteStorageTarget{ID: "integration", Name: "integration", Driver: driver, Host: os.Getenv("REMOTE_TEST_HOST"), Port: port, Database: os.Getenv("REMOTE_TEST_DATABASE"), Username: os.Getenv("REMOTE_TEST_USER"), Password: os.Getenv("REMOTE_TEST_PASSWORD"), TLSMode: os.Getenv("REMOTE_TEST_TLS"), CreateTable: true, Table: fmt.Sprintf("remote_it_%d", time.Now().UnixNano()), Points: []config.HistoryPointRef{{DeviceKey: "d", Metric: "p"}}}
	target.ApplyDefaults()
	opened, e := OpenSQL(target)
	if e != nil {
		t.Fatal(SafeError(e, target))
	}
	a := opened.(*SQLAdapter)
	defer a.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if e = a.Prepare(ctx); e != nil {
		t.Fatal(SafeError(e, target))
	}
	defer func() {
		cleanup, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		if _, e := a.db.ExecContext(cleanup, "DROP TABLE "+a.table()); e != nil {
			t.Errorf("test table cleanup: %s", SafeError(e, target))
		}
	}()
	if driver == "mysql" {
		var client, connection, results string
		if e = a.db.QueryRowContext(ctx, "SELECT @@character_set_client,@@character_set_connection,@@character_set_results").Scan(&client, &connection, &results); e != nil {
			t.Fatal(SafeError(e, target))
		}
		if client != "utf8mb4" || connection != "utf8mb4" || results != "utf8mb4" {
			t.Fatalf("unexpected MySQL charset: %s/%s/%s", client, connection, results)
		}
	}
	records := []Record{testRecord(t, uint64(math.MaxUint64)), testRecord(t, -1.25), testRecord(t, "中文文本🔔"), testRecord(t, true), testRecord(t, nil)}
	for i := 0; i < 200; i++ {
		records = append(records, testRecord(t, i))
	}
	for i := 0; i < 2; i++ {
		if e = a.Write(ctx, records); e != nil {
			t.Fatal(SafeError(e, target))
		}
	}
	var count int
	if e = a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+a.table()).Scan(&count); e != nil || count != len(records) {
		t.Fatalf("idempotency count=%d err=%s", count, SafeError(e, target))
	}
	var integer string
	if e = a.db.QueryRowContext(ctx, "SELECT integer_value FROM "+a.table()+" WHERE record_id="+a.placeholder(1), records[0].ID).Scan(&integer); e != nil || integer != "18446744073709551615" {
		t.Fatalf("integer=%s err=%s", integer, SafeError(e, target))
	}
	var text string
	if e = a.db.QueryRowContext(ctx, "SELECT text_value FROM "+a.table()+" WHERE record_id="+a.placeholder(1), records[2].ID).Scan(&text); e != nil || text != "中文文本🔔" {
		t.Fatalf("text round trip=%q err=%s", text, SafeError(e, target))
	}
	a.target.CreateTable = false
	if e = a.Prepare(ctx); e != nil {
		t.Fatal("read-only existing table verification", SafeError(e, target))
	}
}
