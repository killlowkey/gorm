package callbacks

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	"gorm.io/gorm/utils"
)

func Query(db *gorm.DB) {
	if db.Error == nil {
		// 构建查询语句
		BuildQuerySQL(db)

		// db.DryRun 为 true 则不进行真实查询，只是为了获取生成后的 SQL 语句
		if !db.DryRun && db.Error == nil {
			// 查询操作
			// 1. db.Statement.SQL.String()：sql 语句，还未绑定参数，select * from users where id = ?
			// 2. db.Statement.Vars：SQL 需要绑定的参数, 10
			rows, err := db.Statement.ConnPool.QueryContext(db.Statement.Context, db.Statement.SQL.String(), db.Statement.Vars...)
			if err != nil {
				db.AddError(err)
				return
			}
			defer func() {
				db.AddError(rows.Close())
			}()
			// 扫描数据库返回的数据到 struct model 中
			gorm.Scan(rows, db, 0)
		}
	}
}

// BuildQuerySQL 构建查询 SQL 语句
// SQL 在 GORM 设计中采用 clause.Interface 进行表示，from、where、join、group by、having、order by、limit、offset 都是 clause.Interface 类型
// 这种设计是为了分而治之，每个 clause.Interface 负责自己的 SQL 构建
// select u.username, c.name from users as a
// left join company as c on a.company_id = c.id
// where c.company_price > 50000 and c.address in ('china', 'us')
// order by c.company_name
func BuildQuerySQL(db *gorm.DB) {
	if db.Statement.Schema != nil {
		for _, c := range db.Statement.Schema.QueryClauses {
			db.Statement.AddClause(c)
		}
	}

	// 还未构建过 SQL 语句
	if db.Statement.SQL.Len() == 0 {
		db.Statement.SQL.Grow(100)
		clauseSelect := clause.Select{Distinct: db.Statement.Distinct}

		// db.Statement.ReflectValue 为 model struct
		if db.Statement.ReflectValue.Kind() == reflect.Struct && db.Statement.ReflectValue.Type() == db.Statement.Schema.ModelType {
			var conds []clause.Expression
			for _, primaryField := range db.Statement.Schema.PrimaryFields {
				if v, isZero := primaryField.ValueOf(db.Statement.Context, db.Statement.ReflectValue); !isZero {
					conds = append(conds, clause.Eq{Column: clause.Column{Table: db.Statement.Table, Name: primaryField.DBName}, Value: v})
				}
			}

			if len(conds) > 0 {
				db.Statement.AddClause(clause.Where{Exprs: conds})
			}
		}

		if len(db.Statement.Selects) > 0 { // 选择的列：select id,name,age
			clauseSelect.Columns = make([]clause.Column, len(db.Statement.Selects))
			for idx, name := range db.Statement.Selects {
				if db.Statement.Schema == nil {
					clauseSelect.Columns[idx] = clause.Column{Name: name, Raw: true}
				} else if f := db.Statement.Schema.LookUpField(name); f != nil {
					clauseSelect.Columns[idx] = clause.Column{Name: f.DBName}
				} else {
					clauseSelect.Columns[idx] = clause.Column{Name: name, Raw: true}
				}
			}
		} else if db.Statement.Schema != nil && len(db.Statement.Omits) > 0 { // 包含忽略字段
			// 获取需要查询的列（去除了忽略字段），比如 user 有 id,name,age 字段，当前忽略 age 字段，只需要查询 id 和 name 字段
			selectColumns, _ := db.Statement.SelectAndOmitColumns(false, false)
			clauseSelect.Columns = make([]clause.Column, 0, len(db.Statement.Schema.DBNames))
			for _, dbName := range db.Statement.Schema.DBNames {
				if v, ok := selectColumns[dbName]; (ok && v) || !ok {
					clauseSelect.Columns = append(clauseSelect.Columns, clause.Column{Table: db.Statement.Table, Name: dbName})
				}
			}
		} else if db.Statement.Schema != nil && db.Statement.ReflectValue.IsValid() { // 大多数情况下，都是走到该接口
			queryFields := db.QueryFields
			// 校验
			if !queryFields {
				switch db.Statement.ReflectValue.Kind() {
				case reflect.Struct:
					queryFields = db.Statement.ReflectValue.Type() != db.Statement.Schema.ModelType
				case reflect.Slice:
					queryFields = db.Statement.ReflectValue.Type().Elem() != db.Statement.Schema.ModelType
				}
			}

			if queryFields {
				stmt := gorm.Statement{DB: db}
				// smaller struct
				// 解析 model
				if err := stmt.Parse(db.Statement.Dest); err == nil && (db.QueryFields || stmt.Schema.ModelType != db.Statement.Schema.ModelType) {
					clauseSelect.Columns = make([]clause.Column, len(stmt.Schema.DBNames))

					// 设置 select 语句中需要 select 字段
					for idx, dbName := range stmt.Schema.DBNames {
						clauseSelect.Columns[idx] = clause.Column{Table: db.Statement.Table, Name: dbName}
					}
				}
			}
		}

		// inline joins
		// select * from users 这种就是内查询
		fromClause := clause.From{}
		if v, ok := db.Statement.Clauses["FROM"].Expression.(clause.From); ok {
			fromClause = v
		}

		// db.Statement.Joins 连接其它表
		if len(db.Statement.Joins) != 0 || len(fromClause.Joins) != 0 {
			// select * 情况
			if len(db.Statement.Selects) == 0 && len(db.Statement.Omits) == 0 && db.Statement.Schema != nil {
				clauseSelect.Columns = make([]clause.Column, len(db.Statement.Schema.DBNames))
				for idx, dbName := range db.Statement.Schema.DBNames {
					clauseSelect.Columns[idx] = clause.Column{Table: db.Statement.Table, Name: dbName}
				}
			}

			// 构建连接查询
			specifiedRelationsName := make(map[string]interface{})
			for _, join := range db.Statement.Joins {
				if db.Statement.Schema != nil {
					var isRelations bool // is relations or raw sql
					var relations []*schema.Relationship
					relation, ok := db.Statement.Schema.Relationships.Relations[join.Name]
					if ok {
						isRelations = true
						relations = append(relations, relation)
					} else {
						// handle nested join like "Manager.Company"
						nestedJoinNames := strings.Split(join.Name, ".")
						if len(nestedJoinNames) > 1 {
							isNestedJoin := true
							gussNestedRelations := make([]*schema.Relationship, 0, len(nestedJoinNames))
							currentRelations := db.Statement.Schema.Relationships.Relations
							for _, relname := range nestedJoinNames {
								// incomplete match, only treated as raw sql
								if relation, ok = currentRelations[relname]; ok {
									gussNestedRelations = append(gussNestedRelations, relation)
									currentRelations = relation.FieldSchema.Relationships.Relations
								} else {
									isNestedJoin = false
									break
								}
							}

							if isNestedJoin {
								isRelations = true
								relations = gussNestedRelations
							}
						}
					}

					if isRelations {
						genJoinClause := func(joinType clause.JoinType, parentTableName string, relation *schema.Relationship) clause.Join {
							tableAliasName := relation.Name
							if parentTableName != clause.CurrentTable {
								tableAliasName = utils.NestedRelationName(parentTableName, tableAliasName)
							}

							columnStmt := gorm.Statement{
								Table: tableAliasName, DB: db, Schema: relation.FieldSchema,
								Selects: join.Selects, Omits: join.Omits,
							}

							selectColumns, restricted := columnStmt.SelectAndOmitColumns(false, false)
							for _, s := range relation.FieldSchema.DBNames {
								if v, ok := selectColumns[s]; (ok && v) || (!ok && !restricted) {
									clauseSelect.Columns = append(clauseSelect.Columns, clause.Column{
										Table: tableAliasName,
										Name:  s,
										Alias: utils.NestedRelationName(tableAliasName, s),
									})
								}
							}

							exprs := make([]clause.Expression, len(relation.References))
							for idx, ref := range relation.References {
								if ref.OwnPrimaryKey {
									exprs[idx] = clause.Eq{
										Column: clause.Column{Table: parentTableName, Name: ref.PrimaryKey.DBName},
										Value:  clause.Column{Table: tableAliasName, Name: ref.ForeignKey.DBName},
									}
								} else {
									if ref.PrimaryValue == "" {
										exprs[idx] = clause.Eq{
											Column: clause.Column{Table: parentTableName, Name: ref.ForeignKey.DBName},
											Value:  clause.Column{Table: tableAliasName, Name: ref.PrimaryKey.DBName},
										}
									} else {
										exprs[idx] = clause.Eq{
											Column: clause.Column{Table: tableAliasName, Name: ref.ForeignKey.DBName},
											Value:  ref.PrimaryValue,
										}
									}
								}
							}

							{
								onStmt := gorm.Statement{Table: tableAliasName, DB: db, Clauses: map[string]clause.Clause{}}
								for _, c := range relation.FieldSchema.QueryClauses {
									onStmt.AddClause(c)
								}

								if join.On != nil {
									onStmt.AddClause(join.On)
								}

								if cs, ok := onStmt.Clauses["WHERE"]; ok {
									if where, ok := cs.Expression.(clause.Where); ok {
										where.Build(&onStmt)

										if onSQL := onStmt.SQL.String(); onSQL != "" {
											vars := onStmt.Vars
											for idx, v := range vars {
												bindvar := strings.Builder{}
												onStmt.Vars = vars[0 : idx+1]
												db.Dialector.BindVarTo(&bindvar, &onStmt, v)
												onSQL = strings.Replace(onSQL, bindvar.String(), "?", 1)
											}

											exprs = append(exprs, clause.Expr{SQL: onSQL, Vars: vars})
										}
									}
								}
							}

							return clause.Join{
								Type:  joinType,
								Table: clause.Table{Name: relation.FieldSchema.Table, Alias: tableAliasName},
								ON:    clause.Where{Exprs: exprs},
							}
						}

						parentTableName := clause.CurrentTable
						for _, rel := range relations {
							// joins table alias like "Manager, Company, Manager__Company"
							nestedAlias := utils.NestedRelationName(parentTableName, rel.Name)
							if _, ok := specifiedRelationsName[nestedAlias]; !ok {
								fromClause.Joins = append(fromClause.Joins, genJoinClause(join.JoinType, parentTableName, rel))
								specifiedRelationsName[nestedAlias] = nil
							}

							if parentTableName != clause.CurrentTable {
								parentTableName = utils.NestedRelationName(parentTableName, rel.Name)
							} else {
								parentTableName = rel.Name
							}
						}
					} else {
						fromClause.Joins = append(fromClause.Joins, clause.Join{
							Expression: clause.NamedExpr{SQL: join.Name, Vars: join.Conds},
						})
					}
				} else {
					fromClause.Joins = append(fromClause.Joins, clause.Join{
						Expression: clause.NamedExpr{SQL: join.Name, Vars: join.Conds},
					})
				}
			}

			db.Statement.AddClause(fromClause)
			db.Statement.Joins = nil
		} else {
			db.Statement.AddClauseIfNotExists(clause.From{})
		}

		db.Statement.AddClauseIfNotExists(clauseSelect)

		db.Statement.Build(db.Statement.BuildClauses...)
	}
}

func Preload(db *gorm.DB) {
	if db.Error == nil && len(db.Statement.Preloads) > 0 {
		if db.Statement.Schema == nil {
			db.AddError(fmt.Errorf("%w when using preload", gorm.ErrModelValueRequired))
			return
		}

		// 获取所有 preload 表名
		preloadMap := parsePreloadMap(db.Statement.Schema, db.Statement.Preloads)
		preloadNames := make([]string, 0, len(preloadMap))
		for key := range preloadMap {
			preloadNames = append(preloadNames, key)
		}
		sort.Strings(preloadNames)

		// 创建新的会话执行
		preloadDB := db.Session(&gorm.Session{Context: db.Statement.Context, NewDB: true, SkipHooks: db.Statement.SkipHooks, Initialized: true})
		db.Statement.Settings.Range(func(k, v interface{}) bool {
			preloadDB.Statement.Settings.Store(k, v)
			return true
		})

		if err := preloadDB.Statement.Parse(db.Statement.Dest); err != nil {
			return
		}
		preloadDB.Statement.ReflectValue = db.Statement.ReflectValue
		preloadDB.Statement.Unscoped = db.Statement.Unscoped

		for _, name := range preloadNames {
			if relations := preloadDB.Statement.Schema.Relationships.EmbeddedRelations[name]; relations != nil {
				db.AddError(preloadEmbedded(preloadDB.Table("").Session(&gorm.Session{Context: db.Statement.Context, SkipHooks: db.Statement.SkipHooks}), relations, db.Statement.Schema, preloadMap[name], db.Statement.Preloads[clause.Associations]))
			} else if rel := preloadDB.Statement.Schema.Relationships.Relations[name]; rel != nil {
				// 执行
				db.AddError(preload(preloadDB.Table("").Session(&gorm.Session{Context: db.Statement.Context, SkipHooks: db.Statement.SkipHooks}), rel, append(db.Statement.Preloads[name], db.Statement.Preloads[clause.Associations]...), preloadMap[name]))
			} else {
				db.AddError(fmt.Errorf("%s: %w for schema %s", name, gorm.ErrUnsupportedRelation, db.Statement.Schema.Name))
			}
		}
	}
}

func AfterQuery(db *gorm.DB) {
	if db.Error == nil && db.Statement.Schema != nil && !db.Statement.SkipHooks && db.Statement.Schema.AfterFind && db.RowsAffected > 0 {
		callMethod(db, func(value interface{}, tx *gorm.DB) bool {
			if i, ok := value.(AfterFindInterface); ok {
				db.AddError(i.AfterFind(tx))
				return true
			}
			return false
		})
	}
}
