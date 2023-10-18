package callbacks

import (
	"gorm.io/gorm"
)

func BeginTransaction(db *gorm.DB) {
	// 当前未关闭默认事务，并且没报错
	if !db.Config.SkipDefaultTransaction && db.Error == nil {
		// 开启事务
		if tx := db.Begin(); tx.Error == nil {
			db.Statement.ConnPool = tx.Statement.ConnPool
			// 设置当前已经开启了事务
			db.InstanceSet("gorm:started_transaction", true)
		} else if tx.Error == gorm.ErrInvalidTransaction {
			tx.Error = nil
		} else {
			db.Error = tx.Error
		}
	}
}

func CommitOrRollbackTransaction(db *gorm.DB) {
	if !db.Config.SkipDefaultTransaction {
		// 判断是否开启了事务
		if _, ok := db.InstanceGet("gorm:started_transaction"); ok {
			//  判断是否是回滚事务
			if db.Error != nil {
				db.Rollback()
			} else {
				db.Commit()
			}

			// 归还连接池
			db.Statement.ConnPool = db.ConnPool
		}
	}
}
