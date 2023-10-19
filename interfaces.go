package gorm

import (
	"context"
	"database/sql"

	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// Dialector GORM database dialector
type Dialector interface {
	// Name 数据库驱动名称
	Name() string
	// Initialize 初始化数据库连接、方言配置、GORM 配置
	Initialize(*DB) error
	// Migrator 数据库表和字段迁移
	Migrator(db *DB) Migrator
	// DataTypeOf 字段数据类型
	DataTypeOf(*schema.Field) string
	// DefaultValueOf 字段默认值
	DefaultValueOf(*schema.Field) clause.Expression
	// BindVarTo 占位符 select * from user id = ?
	BindVarTo(writer clause.Writer, stmt *Statement, v interface{})
	// QuoteTo 写入值
	QuoteTo(clause.Writer, string)
	// Explain 根据 sql 和 var 整合成一条完整 SQL
	Explain(sql string, vars ...interface{}) string
}

// Plugin GORM plugin interface
// GORM 插件接口
type Plugin interface {
	// Name 插件名称
	Name() string
	// Initialize 插件行为，用于对数据库进行配置
	Initialize(*DB) error
}

type ParamsFilter interface {
	ParamsFilter(ctx context.Context, sql string, params ...interface{}) (string, []interface{})
}

// ConnPool db conns pool interface
type ConnPool interface {
	// PrepareContext 预编译 SQL，得到 Statement
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	// ExecContext 执行语句，一般而言是插入、开启关闭事务等
	// 1. ctx: 可以指定超时时间，中断执行
	// 2. query: 执行的语句
	// 3. args: 语句中绑定的值
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	// QueryContext 查询一条或多条记录
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	// QueryRowContext 查询一条记录
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// SavePointerDialectorInterface save pointer interface
// 事务，一个大的事务可以拆分为多个回滚点，防止小部分错误，导致整个事务回滚
type SavePointerDialectorInterface interface {
	// SavePoint 保存当前事务为指定回滚点
	SavePoint(tx *DB, name string) error
	// RollbackTo 回滚事务到指定回滚点
	RollbackTo(tx *DB, name string) error
}

// TxBeginner tx beginner
type TxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// ConnPoolBeginner conn pool beginner
type ConnPoolBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (ConnPool, error)
}

// TxCommitter tx committer
type TxCommitter interface {
	Commit() error
	Rollback() error
}

// Tx sql.Tx interface
type Tx interface {
	ConnPool
	TxCommitter
	StmtContext(ctx context.Context, stmt *sql.Stmt) *sql.Stmt
}

// Valuer gorm valuer interface
type Valuer interface {
	GormValue(context.Context, *DB) clause.Expr
}

// GetDBConnector SQL db connector
type GetDBConnector interface {
	GetDBConn() (*sql.DB, error)
}

// GetDBConnectorWithContext represents SQL db connector which takes into
// account the current database context
type GetDBConnectorWithContext interface {
	GetDBConnWithContext(db *DB) (*sql.DB, error)
}

// Rows rows interface
type Rows interface {
	// Columns 列值
	Columns() ([]string, error)
	// ColumnTypes 列类型
	ColumnTypes() ([]*sql.ColumnType, error)
	// Next 是否有下一行数据
	Next() bool
	// Scan 扫描返回值到目标值
	Scan(dest ...interface{}) error
	// Err 执行期间遇到的错误
	Err() error
	// Close 关闭
	Close() error
}

type ErrorTranslator interface {
	Translate(err error) error
}
