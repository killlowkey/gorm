package callbacks

import (
	"gorm.io/gorm"
)

var (
	// INSERT users("name","age") VALUES('ray',10)
	createClauses = []string{"INSERT", "VALUES", "ON CONFLICT"}
	queryClauses  = []string{"SELECT", "FROM", "WHERE", "GROUP BY", "ORDER BY", "LIMIT", "FOR"}
	updateClauses = []string{"UPDATE", "SET", "WHERE"}
	deleteClauses = []string{"DELETE", "FROM", "WHERE"}
)

type Config struct {
	LastInsertIDReversed bool
	CreateClauses        []string
	QueryClauses         []string
	UpdateClauses        []string
	DeleteClauses        []string
}

// RegisterDefaultCallbacks 注册事件回调函数
func RegisterDefaultCallbacks(db *gorm.DB, config *Config) {
	enableTransaction := func(db *gorm.DB) bool {
		return !db.SkipDefaultTransaction
	}

	if len(config.CreateClauses) == 0 {
		config.CreateClauses = createClauses
	}
	if len(config.QueryClauses) == 0 {
		config.QueryClauses = queryClauses
	}
	if len(config.DeleteClauses) == 0 {
		config.DeleteClauses = deleteClauses
	}
	if len(config.UpdateClauses) == 0 {
		config.UpdateClauses = updateClauses
	}

	// 当调用 gorm 开启事务方法时，会调用如下流程的方法
	// 后续也可以通过如下方式来注册回调函数
	// 1. db.Callback().Create().Before("xx")：在 xx name 之前注册
	// 2. db.Callback().Create().After("xx")：在 xx name 之后注册
	// 3. db.Callback().Create().Register("xx"): 注册 xx name 的 callback
	createCallback := db.Callback().Create()
	createCallback.Match(enableTransaction).Register("gorm:begin_transaction", BeginTransaction)
	createCallback.Register("gorm:before_create", BeforeCreate) // 调用接口
	createCallback.Register("gorm:save_before_associations", SaveBeforeAssociations(true))
	createCallback.Register("gorm:create", Create(config)) // 主处理逻辑
	createCallback.Register("gorm:save_after_associations", SaveAfterAssociations(true))
	createCallback.Register("gorm:after_create", AfterCreate)
	createCallback.Match(enableTransaction).Register("gorm:commit_or_rollback_transaction", CommitOrRollbackTransaction) // 是否提交事务
	createCallback.Clauses = config.CreateClauses

	// select 查询语句
	queryCallback := db.Callback().Query()
	queryCallback.Register("gorm:query", Query) // 主处理逻辑
	queryCallback.Register("gorm:preload", Preload)
	queryCallback.Register("gorm:after_query", AfterQuery)
	queryCallback.Clauses = config.QueryClauses

	// delete 语句
	deleteCallback := db.Callback().Delete()
	deleteCallback.Match(enableTransaction).Register("gorm:begin_transaction", BeginTransaction)
	deleteCallback.Register("gorm:before_delete", BeforeDelete)
	deleteCallback.Register("gorm:delete_before_associations", DeleteBeforeAssociations)
	deleteCallback.Register("gorm:delete", Delete(config)) // 主处理逻辑
	deleteCallback.Register("gorm:after_delete", AfterDelete)
	deleteCallback.Match(enableTransaction).Register("gorm:commit_or_rollback_transaction", CommitOrRollbackTransaction)
	deleteCallback.Clauses = config.DeleteClauses

	// update 语句
	updateCallback := db.Callback().Update()
	updateCallback.Match(enableTransaction).Register("gorm:begin_transaction", BeginTransaction)
	updateCallback.Register("gorm:setup_reflect_value", SetupUpdateReflectValue)
	updateCallback.Register("gorm:before_update", BeforeUpdate)
	updateCallback.Register("gorm:save_before_associations", SaveBeforeAssociations(false))
	updateCallback.Register("gorm:update", Update(config)) // 主处理逻辑
	updateCallback.Register("gorm:save_after_associations", SaveAfterAssociations(false))
	updateCallback.Register("gorm:after_update", AfterUpdate)
	updateCallback.Match(enableTransaction).Register("gorm:commit_or_rollback_transaction", CommitOrRollbackTransaction)
	updateCallback.Clauses = config.UpdateClauses

	rowCallback := db.Callback().Row()
	rowCallback.Register("gorm:row", RowQuery)
	rowCallback.Clauses = config.QueryClauses

	rawCallback := db.Callback().Raw()
	rawCallback.Register("gorm:raw", RawExec)
	rawCallback.Clauses = config.QueryClauses
}
