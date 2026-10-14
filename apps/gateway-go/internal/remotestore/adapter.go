package remotestore

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	mssql "github.com/microsoft/go-mssqldb"
	"weikong-iot-platform/apps/gateway-go/internal/config"
)

// Adapter is the extension seam for DM/Kingbase/other database dialects.
// Queuing, persistence and acknowledgement never depend on a vendor driver.
type Adapter interface {
	Prepare(context.Context) error
	Write(context.Context, []Record) error
	Close() error
}
type Factory func(config.RemoteStorageTarget) (Adapter, error)
type SQLAdapter struct {
	db     *sql.DB
	target config.RemoteStorageTarget
}

var factories = map[string]Factory{"mysql": OpenSQL, "sqlserver": OpenSQL, "postgresql": OpenSQL}

func Open(t config.RemoteStorageTarget) (Adapter, error) {
	f := factories[t.Driver]
	if f == nil {
		return nil, fmt.Errorf("数据库适配器未安装")
	}
	return f(t)
}

func OpenSQL(t config.RemoteStorageTarget) (Adapter, error) {
	if err := t.ValidateConnection(); err != nil {
		return nil, err
	}
	var db *sql.DB
	switch t.Driver {
	case "mysql":
		c := mysql.NewConfig()
		c.User = t.Username
		c.Passwd = t.Password
		c.Net = "tcp"
		c.Addr = net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
		c.DBName = t.Database
		c.Timeout = time.Duration(t.TimeoutSeconds) * time.Second
		c.ReadTimeout = c.Timeout
		c.WriteTimeout = c.Timeout
		// NewConnector consumes Config directly: Params are server variables,
		// not DSN options. Use Charset so the driver issues SET NAMES utf8mb4.
		if err := c.Apply(mysql.Charset("utf8mb4", "")); err != nil {
			return nil, err
		}
		if t.TLSMode == "verify-full" {
			c.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: t.Host}
			if t.CAFile != "" {
				raw, err := os.ReadFile(t.CAFile)
				if err != nil {
					return nil, err
				}
				pool := x509.NewCertPool()
				if !pool.AppendCertsFromPEM(raw) {
					return nil, fmt.Errorf("CA 证书无效")
				}
				c.TLS.RootCAs = pool
			}
		}
		connector, err := mysql.NewConnector(c)
		if err != nil {
			return nil, err
		}
		db = sql.OpenDB(connector)
	case "postgresql", "sqlserver":
		u := url.URL{Scheme: t.Driver, Host: net.JoinHostPort(t.Host, strconv.Itoa(t.Port)), User: url.UserPassword(t.Username, t.Password)}
		q := url.Values{}
		if t.Driver == "postgresql" {
			u.Path = "/" + t.Database
			q.Set("sslmode", map[bool]string{true: "verify-full", false: "disable"}[t.TLSMode == "verify-full"])
			q.Set("connect_timeout", strconv.Itoa(t.TimeoutSeconds))
			if t.CAFile != "" {
				q.Set("sslrootcert", t.CAFile)
			}
		} else {
			q.Set("database", t.Database)
			q.Set("encrypt", map[bool]string{true: "true", false: "disable"}[t.TLSMode == "verify-full"])
			q.Set("TrustServerCertificate", "false")
			q.Set("connection timeout", strconv.Itoa(t.TimeoutSeconds))
			if t.CAFile != "" {
				q.Set("certificate", t.CAFile)
			}
		}
		u.RawQuery = q.Encode()
		if t.Driver == "postgresql" {
			connector, err := pq.NewConnector(u.String())
			if err != nil {
				return nil, err
			}
			db = sql.OpenDB(connector)
		} else {
			connector, err := mssql.NewConnector(u.String())
			if err != nil {
				return nil, err
			}
			db = sql.OpenDB(connector)
		}
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &SQLAdapter{db: db, target: t}, nil
}
func (a *SQLAdapter) Close() error { return a.db.Close() }
func (a *SQLAdapter) quote(s string) string {
	switch a.target.Driver {
	case "mysql":
		return "`" + s + "`"
	case "sqlserver":
		return "[" + s + "]"
	default:
		return `"` + s + `"`
	}
}
func (a *SQLAdapter) table() string {
	if a.target.Schema != "" {
		return a.quote(a.target.Schema) + "." + a.quote(a.target.Table)
	}
	return a.quote(a.target.Table)
}

const columns = "record_id,gateway_key,device_key,metric,sampled_ms,received_ms,source_ms,value_type,value_json,integer_value,float_value,boolean_value,text_value,actual_type,quality,unit,diagnostic"

func (a *SQLAdapter) ddl() string {
	textType, floatType := "TEXT", "DOUBLE PRECISION"
	if a.target.Driver == "mysql" {
		textType = "LONGTEXT"
		floatType = "DOUBLE"
	}
	if a.target.Driver == "sqlserver" {
		textType = "NVARCHAR(MAX)"
		floatType = "FLOAT"
	}
	varchar := "VARCHAR"
	if a.target.Driver == "sqlserver" {
		varchar = "NVARCHAR"
	}
	fields := fmt.Sprintf("record_id VARCHAR(64) NOT NULL PRIMARY KEY,gateway_key %s(256) NOT NULL,device_key %s(256) NOT NULL,metric %s(256) NOT NULL,sampled_ms BIGINT NOT NULL,received_ms BIGINT,source_ms BIGINT,value_type VARCHAR(24) NOT NULL,value_json %s NOT NULL,integer_value VARCHAR(32),float_value %s,boolean_value SMALLINT,text_value %s,actual_type %s(128),quality VARCHAR(24),unit %s(128),diagnostic %s", varchar, varchar, varchar, textType, floatType, textType, varchar, varchar, textType)
	if a.target.Driver == "sqlserver" {
		return "IF OBJECT_ID(N'" + a.target.Schema + "." + a.target.Table + "', N'U') IS NULL CREATE TABLE " + a.table() + " (" + fields + ")"
	}
	suffix := ""
	if a.target.Driver == "mysql" {
		suffix = " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
	}
	return "CREATE TABLE IF NOT EXISTS " + a.table() + " (" + fields + ")" + suffix
}
func (a *SQLAdapter) Prepare(ctx context.Context) error {
	if err := a.db.PingContext(ctx); err != nil {
		return err
	}
	if a.target.CreateTable {
		if _, err := a.db.ExecContext(ctx, a.ddl()); err != nil {
			return fmt.Errorf("创建专用表失败: %w", err)
		}
	}
	// Read-only table validation also runs when using an existing table.
	rows, err := a.db.QueryContext(ctx, "SELECT "+columns+" FROM "+a.table()+" WHERE 1=0")
	if err != nil {
		return fmt.Errorf("目标表字段不匹配: %w", err)
	}
	rows.Close()
	var query string
	var args []any
	switch a.target.Driver {
	case "mysql":
		query = "SELECT COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND CONSTRAINT_NAME='PRIMARY' ORDER BY ORDINAL_POSITION"
		args = []any{a.target.Database, a.target.Table}
	case "postgresql":
		query = "SELECT k.column_name FROM information_schema.table_constraints t JOIN information_schema.key_column_usage k ON t.constraint_catalog=k.constraint_catalog AND t.constraint_schema=k.constraint_schema AND t.constraint_name=k.constraint_name AND t.table_name=k.table_name WHERE t.table_schema=$1 AND t.table_name=$2 AND t.constraint_type='PRIMARY KEY' ORDER BY k.ordinal_position"
		args = []any{a.target.Schema, a.target.Table}
	case "sqlserver":
		query = "SELECT k.COLUMN_NAME FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS t JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE k ON t.CONSTRAINT_NAME=k.CONSTRAINT_NAME AND t.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA WHERE t.TABLE_SCHEMA=@p1 AND t.TABLE_NAME=@p2 AND t.CONSTRAINT_TYPE='PRIMARY KEY' ORDER BY k.ORDINAL_POSITION"
		args = []any{a.target.Schema, a.target.Table}
	}
	rows, err = a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return err
		}
		keys = append(keys, key)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(keys) != 1 || keys[0] != "record_id" {
		return fmt.Errorf("目标表必须以 record_id 为唯一主键，以保证补传去重")
	}
	rows.Close()
	if a.target.Driver == "mysql" {
		var engine string
		if err = a.db.QueryRowContext(ctx, "SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?", a.target.Database, a.target.Table).Scan(&engine); err != nil {
			return err
		}
		if !strings.EqualFold(engine, "InnoDB") {
			return fmt.Errorf("MySQL 专用表必须使用 InnoDB 以保证事务提交与补传一致性")
		}
	}
	return nil
}
func (a *SQLAdapter) placeholder(n int) string {
	switch a.target.Driver {
	case "postgresql":
		return "$" + strconv.Itoa(n)
	case "sqlserver":
		return "@p" + strconv.Itoa(n)
	default:
		return "?"
	}
}
func (a *SQLAdapter) insertSQL() string { return a.batchInsertSQL(1) }
func (a *SQLAdapter) batchInsertSQL(count int) string {
	width := len(strings.Split(columns, ","))
	values := make([]string, count)
	for row := range values {
		p := make([]string, width)
		for col := range p {
			p[col] = a.placeholder(row*width + col + 1)
		}
		values[row] = "(" + strings.Join(p, ",") + ")"
	}
	valueSQL := strings.Join(values, ",")
	base := "INSERT INTO " + a.table() + " (" + columns + ") "
	switch a.target.Driver {
	case "mysql":
		return base + "VALUES " + valueSQL + " ON DUPLICATE KEY UPDATE record_id=record_id"
	case "postgresql":
		return base + "VALUES " + valueSQL + " ON CONFLICT (record_id) DO NOTHING"
	default:
		return base + "SELECT incoming.* FROM (VALUES " + valueSQL + ") AS incoming (" + columns + ") WHERE NOT EXISTS (SELECT 1 FROM " + a.table() + " WITH (UPDLOCK,HOLDLOCK) WHERE record_id=incoming.record_id)"
	}
}
func (a *SQLAdapter) verifySQL(count int) string {
	p := make([]string, count)
	for i := range p {
		p[i] = a.placeholder(i + 1)
	}
	return "SELECT record_id,value_json FROM " + a.table() + " WHERE record_id IN (" + strings.Join(p, ",") + ")"
}
func (a *SQLAdapter) Write(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// True multi-row batches: avoid 2 round trips per point on slow links.
	// 100 * 17 parameters stays below SQL Server's 2100-parameter limit.
	for start := 0; start < len(records); start += 100 {
		end := start + 100
		if end > len(records) {
			end = len(records)
		}
		chunk := records[start:end]
		args := make([]any, 0, len(chunk)*17)
		ids := make([]any, 0, len(chunk))
		expected := map[string]string{}
		for _, r := range chunk {
			args = append(args, r.Args()...)
			ids = append(ids, r.ID)
			expected[r.ID] = r.ValueJSON
		}
		if _, err = tx.ExecContext(ctx, a.batchInsertSQL(len(chunk)), args...); err != nil {
			return err
		}
		// MySQL's duplicate clause can match another unique key. Check actual ID
		// and payload before acknowledging; never silently drop such a collision.
		rows, e := tx.QueryContext(ctx, a.verifySQL(len(chunk)), ids...)
		if e != nil {
			return e
		}
		for rows.Next() {
			var id, payload string
			if e = rows.Scan(&id, &payload); e != nil {
				rows.Close()
				return e
			}
			if wanted, ok := expected[id]; !ok || wanted != payload {
				rows.Close()
				return fmt.Errorf("目标表记录编号冲突，已停止补传")
			}
			delete(expected, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(expected) > 0 {
			return fmt.Errorf("目标表未完整保存本批记录，已停止补传")
		}
	}
	return tx.Commit()
}

func SafeError(err error, t config.RemoteStorageTarget) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	// Drivers may include connection URLs in errors; redact literal and escaped secrets.
	if t.Password != "" {
		for _, v := range []string{t.Password, url.QueryEscape(t.Password), url.PathEscape(t.Password), strings.TrimPrefix(url.UserPassword("", t.Password).String(), ":")} {
			s = strings.ReplaceAll(s, v, "***")
		}
	}
	if len(s) > 800 {
		s = s[:800]
	}
	return s
}
