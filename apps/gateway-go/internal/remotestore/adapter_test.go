package remotestore

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

func TestAdaptersTransactionAndReplay(t *testing.T) {
	for _, driver := range []string{"mysql", "sqlserver", "postgresql"} {
		for _, failure := range []string{"", "commit", "collision", "exec"} {
			t.Run(driver+"/"+failure, func(t *testing.T) {
				db, mock, e := sqlmock.New()
				if e != nil {
					t.Fatal(e)
				}
				defer db.Close()
				target := config.RemoteStorageTarget{Driver: driver, Table: "samples"}
				target.ApplyDefaults()
				a := &SQLAdapter{db: db, target: target}
				r := testRecord(t, 12)
				mock.ExpectBegin()
				exec := mock.ExpectExec(regexp.QuoteMeta(a.insertSQL()))
				if failure == "exec" {
					exec.WillReturnError(errors.New("server disconnected"))
					mock.ExpectRollback()
				} else {
					exec.WillReturnResult(sqlmock.NewResult(0, 1))
					payload := r.ValueJSON
					if failure == "collision" {
						payload = "different"
					}
					mock.ExpectQuery(regexp.QuoteMeta(a.verifySQL(1))).WithArgs(r.ID).WillReturnRows(sqlmock.NewRows([]string{"record_id", "value_json"}).AddRow(r.ID, payload))
					if failure == "collision" {
						mock.ExpectRollback()
					} else {
						commit := mock.ExpectCommit()
						if failure == "commit" {
							commit.WillReturnError(errors.New("commit outcome unknown"))
						}
					}
				}
				e = a.Write(context.Background(), []Record{r})
				if (failure == "") != (e == nil) {
					t.Fatalf("failure=%s err=%v", failure, e)
				}
				if e = mock.ExpectationsWereMet(); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}
func TestAdaptersPrepareReadOnlyAndPrimaryKey(t *testing.T) {
	for _, driver := range []string{"mysql", "sqlserver", "postgresql"} {
		for _, badKey := range []bool{false, true} {
			t.Run(driver+map[bool]string{true: "bad-key", false: "valid"}[badKey], func(t *testing.T) {
				db, mock, e := sqlmock.New(sqlmock.MonitorPingsOption(true))
				if e != nil {
					t.Fatal(e)
				}
				defer db.Close()
				target := config.RemoteStorageTarget{Driver: driver, Database: "test", Table: "samples"}
				target.ApplyDefaults()
				a := &SQLAdapter{db: db, target: target}
				mock.ExpectPing()
				mock.ExpectQuery(regexp.QuoteMeta("SELECT " + columns + " FROM " + a.table() + " WHERE 1=0")).WillReturnRows(sqlmock.NewRows(strings.Split(columns, ",")))
				key := "record_id"
				if badKey {
					key = "another"
				}
				mock.ExpectQuery("SELECT .*COLUMN_NAME|SELECT .*column_name").WillReturnRows(sqlmock.NewRows([]string{"key"}).AddRow(key))
				if driver == "mysql" && !badKey {
					mock.ExpectQuery("SELECT ENGINE FROM").WillReturnRows(sqlmock.NewRows([]string{"engine"}).AddRow("InnoDB"))
				}
				e = a.Prepare(context.Background())
				if badKey != (e != nil) {
					t.Fatalf("keycheck: %v", e)
				}
				if e = mock.ExpectationsWereMet(); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}
func TestAdapterSecretsAndIdentifiers(t *testing.T) {
	target := config.RemoteStorageTarget{Password: "a b&c@x"}
	s := SafeError(errors.New("a b&c@x a+b%26c%40x a%20b&c@x"), target)
	if strings.Contains(s, target.Password) || strings.Contains(s, "a+b%26c%40x") {
		t.Fatal("secret leaked")
	}
	encoded := strings.TrimPrefix(url.UserPassword("", target.Password).String(), ":")
	if strings.Contains(SafeError(errors.New("postgres://writer:"+encoded+"@db"), target), encoded) {
		t.Fatal("userinfo password escaped form leaked")
	}
	for _, driver := range []string{"mysql", "sqlserver", "postgresql"} {
		target := config.RemoteStorageTarget{ID: "t", Name: "test", Host: "127.0.0.1", Driver: driver, Database: "test", Username: "test", Table: "x;DROP TABLE y", Points: []config.HistoryPointRef{{DeviceKey: "d", Metric: "p"}}}
		target.ApplyDefaults()
		if a, e := OpenSQL(target); e == nil {
			a.Close()
			t.Fatal("unsafe identifier accepted")
		}
	}
}

func TestSQLServerSplitsBatchBelowParameterLimit(t *testing.T) {
	db, mock, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	target := config.RemoteStorageTarget{Driver: "sqlserver", Table: "samples"}
	target.ApplyDefaults()
	a := &SQLAdapter{db: db, target: target}
	var records []Record
	for i := 0; i < 205; i++ {
		records = append(records, testRecord(t, i))
	}
	mock.ExpectBegin()
	for start := 0; start < len(records); start += 100 {
		end := start + 100
		if end > len(records) {
			end = len(records)
		}
		chunk := records[start:end]
		query := a.batchInsertSQL(len(chunk))
		if strings.Contains(query, "@p2101") {
			t.Fatal("SQL Server parameter limit exceeded")
		}
		mock.ExpectExec(regexp.QuoteMeta(query)).WillReturnResult(sqlmock.NewResult(0, int64(len(chunk))))
		rows := sqlmock.NewRows([]string{"record_id", "value_json"})
		for _, r := range chunk {
			rows.AddRow(r.ID, r.ValueJSON)
		}
		mock.ExpectQuery(regexp.QuoteMeta(a.verifySQL(len(chunk)))).WillReturnRows(rows)
	}
	mock.ExpectCommit()
	if e = a.Write(context.Background(), records); e != nil {
		t.Fatal(e)
	}
	if e = mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
