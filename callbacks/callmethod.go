package callbacks

import (
	"reflect"

	"gorm.io/gorm"
)

// callMethod 包装方法，方法内部创建新的 Session，用于执行语句，每个 Session 之间互不影响
func callMethod(db *gorm.DB, fc func(value interface{}, tx *gorm.DB) bool) {
	// 创建 Session
	tx := db.Session(&gorm.Session{NewDB: true})
	// 调用回调函数，回调函数使用 tx 执行语句
	if called := fc(db.Statement.ReflectValue.Interface(), tx); !called {
		// 目标值
		switch db.Statement.ReflectValue.Kind() {
		case reflect.Slice, reflect.Array: // 切片、数组
			db.Statement.CurDestIndex = 0
			for i := 0; i < db.Statement.ReflectValue.Len(); i++ {
				if value := reflect.Indirect(db.Statement.ReflectValue.Index(i)); value.CanAddr() {
					fc(value.Addr().Interface(), tx)
				} else {
					db.AddError(gorm.ErrInvalidValue)
					return
				}
				db.Statement.CurDestIndex++
			}
		case reflect.Struct: // 结构体
			if db.Statement.ReflectValue.CanAddr() {
				fc(db.Statement.ReflectValue.Addr().Interface(), tx)
			} else {
				db.AddError(gorm.ErrInvalidValue)
			}
		}
	}
}
