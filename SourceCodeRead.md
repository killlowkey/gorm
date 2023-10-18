## 目录结构
1. callback: 存储回调函数，比如 create、update、insert、delete 等
2. clause: 存储 SQL 语句的片段，比如 select、from、where、group by、having、order by、limit 等
   > 分而治之的思想，每个 SQL 语句都拆分成若干个片段，然后根据片段的执行顺序依次执行
   > 
   > `expression.go` 包含一个 `clause.Expression` 顶级接口，用于构建语句（select、update、insert、delete）和条件表达式，and、or、in、
   > not in、like、not like、between、not between、is null、is not null、in、not in、like、not
   1. SELECT: 依次调用 SELECT、FROM、WHERE、GROUP BY、ORDER BY、LIMIT、FOR 
   2. UPDATE: 依次调用 UPDATE、SET、WHERE 
   3. INSERT: 依次调用 INSERT、VALUES、ON CONFLICT 
   4. DELETE: 依次调用 DELETE、FROM、WHERE
3. logger: 日志相关操作，记录耗时、执行的 SQL 语句等
4. migrator: 数据库迁移相关操作，比如创建表、添加字段、修改字段、删除表等
5. schema: 存储数据库的表结构，比如表名、字段名、字段类型、索引、外键等
6. tests: 测试文件
7. utils: 工具类
8. association: 关系操作 hasOne、belongTo、HasMany、ManyToMany
9. callbacks.go：select、update、insert、delete 等回调函数统一入口
10. chainable_api.go: 流式操作，进行语句整合操作，调用里面任何方法，返回 db 对象，返回的对象里面还可以调用对应的流式操作
    > select、table、where、order by、limit、offset、group by、having、join、left join、right join、inner join、cross join、
    > where、or、and、not、in、not in、like、not like、between、not between、is null、is not null、in、not in、like、not like
11. errors.go: gorm 使用的错误类型
12. finisher_api.go: 终止操作，调用里面任何一个方法， db 对象会执行对应的 sql 语句并返回值
    > first、last、take、create、update、delete、exec、beginTransaction、commit、rollback 等
    > 可以通过 DB 的 Error 和 RowsAffected 判断执行结果
13. gorm.go: 配置、数据库连接、session 创建等
14. interfaces.go: 全局接口定义
15. migrator.go: 迁移操作接口定义，交由具体的数据库驱动来实现
16. model.go: 表通用字段，id、created_at、updated_at、deleted_at
17. prepare_stmt.go: 预编译语句
18. scan.go: 扫描数据库返回的数据到结构体
19. scan_delete.go:
20. statement.go: 语句，可以理解每个 SQL 对应一个 statement

## 设计核心思想
GORM 主要是围绕着下面几个类型来进行设计
1. gorm.callback: 实现数据库的 Insert、Update、Select、Delete 操作和自定义的回调操作
   > 比如一个 Insert 操作会有一组 callback 操作，在实际运行中，所有的 callback 都会被封装成 func(db *gorm.DB) 函数，插入前需要开启事务，
   > 调用一些 Hook 函数，插入后需要检测是否提交事务，包含具体的插入也是通过 callback 来实现。
   > 不同的数据库，在 SQL 语法和数据类型上有些许差异，这些差异之处被称为“方言”，针对这些方言，GORM 通过数据库驱动实现 Dialector 接口来定制化某些操作
2. clause.Interface: 每个 SQL 片段对应一个 clause，负责自身的 SQL 语句构建（主流设计思想）
   > 比如 select、from、where、group by、having、order by、limit 等
3. clause.Builder: 构建 SQL 语句，由数据库驱动来实现
   ```
   MySQL 使用 `` 来表示表名和字段名: SELECT  `user`.`id`, `user`.`name` FROM `user`
   PostgreSQL 使用 "" 来表示表名和字段名: SELECT  "user"."id", "user"."name" FROM "user"
   ```
4. gorm.Dialector: 交由数据库驱动来实现

## 源码阅读步骤
1. 阅读 tests 目录中测试代码，熟悉 gorm 一些概念
2. 阅读根目录代码 interfaces.go、gorm.go、model.go、errors.go、statement.go 等
3. 阅读 callbacks 和 clause 目录

## gorm.Open 流程分析
如以下代码所示，通过 gorm 创建 sqlite 数据库连接
> sqlite 在 gorm 中有两种数据库驱动：CGO和纯Go实现
```go
 db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
```
1. `sqlite.Open("test.db")`: 来获取 sqlite 的 Dialector 实现，这是数据库和 GORM 沟通的桥梁
2. `&gorm.Config{}`: GORM 全局配置

之后进入 `gorm.Open` 方法来分析具体的 open 流程
1. 收集 Open 方法传入的配置和数据库驱动配置，组合成一个 `gorm.Config` 对象
2. 当某些配置为空时，初始化默认配置
3. 调用 initializeCallbacks 方法：初始化 callback，为 select、update、delete 等语句注册 processor
4. **调用 Dialector#Initialize 方法：数据库驱动进行初始化（连接数据库、方言配置等）**
   > 在每个数据库驱动的 Initialize 方法中，都会调用 `callbacks.RegisterDefaultCallbacks` 来注册对应的语句的 callback，数据库不同每个语句（SELECT、UPDATE 等）对应的 Clause 也不同
5. 设置 Statement，进行一些后置操作，比如 ping 数据库

## DB.Create 流程分析
如下代码，连接 sqlite 数据库，并创建一条数据
```go
  db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
  if err != nil {
    panic("failed to connect database")
  }

  // Create
  db.Create(&Product{Code: "D42", Price: 100})
```

Create 代码如下所示
1. CreateBatchSize 大于 0，则调用 CreateInBatches 进行批量插入
2. 获取数据库实例，可以理解为是一个事务，每次创建都是通过新事务来创建
3. 设置 Dest 值，即要插入的数据
4. 获取 create 操作对应的 processor，然后执行（最终会进入 `processor#Execute` 方法）
```go
func (db *DB) Create(value interface{}) (tx *DB) {
	if db.CreateBatchSize > 0 {
		return db.CreateInBatches(value, db.CreateBatchSize)
	}

	tx = db.getInstance()
	// 设置目标值
	tx.Statement.Dest = value
	// 执行 sql
	return tx.callbacks.Create().Execute(tx)
}
```

processor#Execute 操作
1. 解析 model 和 dest
2. 调用 Create 语句对应的 func(db *gorm.DB) 函数：开启事务、生成 SQL、执行、提交事务等操作
3. 进行收尾工作：记录日志
```go
func (p *processor) Execute(db *DB) *DB {
	// call scopes
	for len(db.Statement.scopes) > 0 {
		db = db.executeScopes()
	}

	var (
		curTime           = time.Now()   // 当前时间
		stmt              = db.Statement // 当前 statement
		resetBuildClauses bool
	)

	if len(stmt.BuildClauses) == 0 {
		stmt.BuildClauses = p.Clauses
		resetBuildClauses = true
	}

	if optimizer, ok := db.Statement.Dest.(StatementModifier); ok {
		optimizer.ModifyStatement(stmt)
	}

	// assign model values
	if stmt.Model == nil {
		stmt.Model = stmt.Dest
	} else if stmt.Dest == nil {
		stmt.Dest = stmt.Model
	}

	// parse model values
	if stmt.Model != nil {
		if err := stmt.Parse(stmt.Model); err != nil && (!errors.Is(err, schema.ErrUnsupportedDataType) || (stmt.Table == "" && stmt.TableExpr == nil && stmt.SQL.Len() == 0)) {
			if errors.Is(err, schema.ErrUnsupportedDataType) && stmt.Table == "" && stmt.TableExpr == nil {
				db.AddError(fmt.Errorf("%w: Table not set, please set it like: db.Model(&user) or db.Table(\"users\")", err))
			} else {
				db.AddError(err)
			}
		}
	}

	// assign stmt.ReflectValue
	if stmt.Dest != nil {
		stmt.ReflectValue = reflect.ValueOf(stmt.Dest)
		for stmt.ReflectValue.Kind() == reflect.Ptr {
			if stmt.ReflectValue.IsNil() && stmt.ReflectValue.CanAddr() {
				stmt.ReflectValue.Set(reflect.New(stmt.ReflectValue.Type().Elem()))
			}

			stmt.ReflectValue = stmt.ReflectValue.Elem()
		}
		if !stmt.ReflectValue.IsValid() {
			db.AddError(ErrInvalidValue)
		}
	}

	// 核心操作：调用每个语句核心操作和回调
	// RegisterDefaultCallbacks 加上自定义注册的回调
	for _, f := range p.fns {
		f(db)
	}

	// 记录日志
	if stmt.SQL.Len() > 0 {
		db.Logger.Trace(stmt.Context, curTime, func() (string, int64) {
			sql, vars := stmt.SQL.String(), stmt.Vars
			if filter, ok := db.Logger.(ParamsFilter); ok {
				sql, vars = filter.ParamsFilter(stmt.Context, stmt.SQL.String(), stmt.Vars...)
			}
			return db.Dialector.Explain(sql, vars...), db.RowsAffected
		}, db.Error)
	}

	if !stmt.DB.DryRun {
		stmt.SQL.Reset()
		stmt.Vars = nil
	}

	if resetBuildClauses {
		stmt.BuildClauses = nil
	}

	return db
}
```

