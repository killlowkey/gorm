package gorm

import (
	"reflect"

	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// Migrator returns migrator
func (db *DB) Migrator() Migrator {
	tx := db.getInstance()

	// apply scopes to migrator
	for len(tx.Statement.scopes) > 0 {
		tx = tx.executeScopes()
	}

	return tx.Dialector.Migrator(tx.Session(&Session{}))
}

// AutoMigrate run auto migration for given models
func (db *DB) AutoMigrate(dst ...interface{}) error {
	return db.Migrator().AutoMigrate(dst...)
}

// ViewOption view option
type ViewOption struct {
	Replace     bool   // If true, exec `CREATE`. If false, exec `CREATE OR REPLACE`
	CheckOption string // optional. e.g. `WITH [ CASCADED | LOCAL ] CHECK OPTION`
	Query       *DB    // required subquery.
}

// ColumnType column type interface
type ColumnType interface {
	Name() string                                         // 字段名称
	DatabaseTypeName() string                             // varchar
	ColumnType() (columnType string, ok bool)             // varchar(64)
	PrimaryKey() (isPrimaryKey bool, ok bool)             // 是否主键
	AutoIncrement() (isAutoIncrement bool, ok bool)       // 是否自增
	Length() (length int64, ok bool)                      // 字段长度
	DecimalSize() (precision int64, scale int64, ok bool) // decimal(10,2)
	Nullable() (nullable bool, ok bool)                   // 是否允许为空
	Unique() (unique bool, ok bool)                       // 是否唯一
	ScanType() reflect.Type                               // 数据库字段类型对应的 Go 类型
	Comment() (value string, ok bool)                     // 字段注释
	DefaultValue() (value string, ok bool)                // 默认值
}

type Index interface {
	Table() string                            // 表名
	Name() string                             // 索引名称
	Columns() []string                        // 索引字段
	PrimaryKey() (isPrimaryKey bool, ok bool) // 是否主键
	Unique() (unique bool, ok bool)           // 是否唯一
	Option() string                           // 索引选项
}

// TableType table type interface
type TableType interface {
	Schema() string                     // Schema
	Name() string                       // Table name
	Type() string                       // Table type
	Comment() (comment string, ok bool) // Table comment
}

// Migrator migrator interface
type Migrator interface {
	// AutoMigrate 迁移
	AutoMigrate(dst ...interface{}) error

	// Database 操作
	CurrentDatabase() string
	FullDataTypeOf(*schema.Field) clause.Expr
	GetTypeAliases(databaseTypeName string) []string

	// Tables
	CreateTable(dst ...interface{}) error
	DropTable(dst ...interface{}) error
	HasTable(dst interface{}) bool
	RenameTable(oldName, newName interface{}) error
	GetTables() (tableList []string, err error)
	TableType(dst interface{}) (TableType, error)

	// Columns
	AddColumn(dst interface{}, field string) error
	DropColumn(dst interface{}, field string) error
	AlterColumn(dst interface{}, field string) error
	MigrateColumn(dst interface{}, field *schema.Field, columnType ColumnType) error
	HasColumn(dst interface{}, field string) bool
	RenameColumn(dst interface{}, oldName, field string) error
	ColumnTypes(dst interface{}) ([]ColumnType, error)

	// Views
	CreateView(name string, option ViewOption) error
	DropView(name string) error

	// Constraints
	CreateConstraint(dst interface{}, name string) error
	DropConstraint(dst interface{}, name string) error
	HasConstraint(dst interface{}, name string) bool

	// Indexes
	CreateIndex(dst interface{}, name string) error
	DropIndex(dst interface{}, name string) error
	HasIndex(dst interface{}, name string) bool
	RenameIndex(dst interface{}, oldName, newName string) error
	GetIndexes(dst interface{}) ([]Index, error)
}
