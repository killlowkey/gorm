package callbacks

import (
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	"gorm.io/gorm/utils"
)

func BeforeDelete(db *gorm.DB) {
	if db.Error == nil && db.Statement.Schema != nil && !db.Statement.SkipHooks && db.Statement.Schema.BeforeDelete {
		callMethod(db, func(value interface{}, tx *gorm.DB) bool {
			if i, ok := value.(BeforeDeleteInterface); ok {
				db.AddError(i.BeforeDelete(tx))
				return true
			}

			return false
		})
	}
}

// DeleteBeforeAssociations 关联删除，比如使用了 hasOne hasMany Many2Many 关联，在删除主表数据时，关联数据也会被删除
func DeleteBeforeAssociations(db *gorm.DB) {
	if db.Error == nil && db.Statement.Schema != nil {
		// 剔除忽略字段，获取最终字段
		selectColumns, restricted := db.Statement.SelectAndOmitColumns(true, false)
		if !restricted {
			return
		}

		for column, v := range selectColumns {
			if !v {
				continue
			}

			// 获取字段关系类型
			rel, ok := db.Statement.Schema.Relationships.Relations[column]
			if !ok {
				continue
			}

			switch rel.Type {
			case schema.HasOne, schema.HasMany: // 一对一、一对多
				// 转为查询条件，删除时需要
				queryConds := rel.ToQueryConditions(db.Statement.Context, db.Statement.ReflectValue)
				// 获取该字段对应的 model
				modelValue := reflect.New(rel.FieldSchema.ModelType).Interface()
				tx := db.Session(&gorm.Session{NewDB: true}).Model(modelValue)
				withoutConditions := false
				if db.Statement.Unscoped {
					tx = tx.Unscoped()
				}

				if len(db.Statement.Selects) > 0 {
					selects := make([]string, 0, len(db.Statement.Selects))
					for _, s := range db.Statement.Selects {
						if s == clause.Associations {
							selects = append(selects, s)
						} else if columnPrefix := column + "."; strings.HasPrefix(s, columnPrefix) {
							selects = append(selects, strings.TrimPrefix(s, columnPrefix))
						}
					}

					if len(selects) > 0 {
						tx = tx.Select(selects)
					}
				}

				for _, cond := range queryConds {
					if c, ok := cond.(clause.IN); ok && len(c.Values) == 0 {
						withoutConditions = true
						break
					}
				}

				// 构建 where 查询
				if !withoutConditions && db.AddError(tx.Clauses(clause.Where{Exprs: queryConds}).Delete(modelValue).Error) != nil {
					return
				}
			case schema.Many2Many: // 多对多 select * from users as u left join company as c on u.company_id = c.id;
				var (
					// 查询条件
					queryConds = make([]clause.Expression, 0, len(rel.References))
					// 外键字段
					foreignFields = make([]*schema.Field, 0, len(rel.References))
					// 引用外键字段
					relForeignKeys = make([]string, 0, len(rel.References))
					// 连接表的 mode
					modelValue = reflect.New(rel.JoinTable.ModelType).Interface()
					// 连接的表
					table = rel.JoinTable.Table
					tx    = db.Session(&gorm.Session{NewDB: true}).Model(modelValue).Table(table)
				)

				// 获取引用字段
				for _, ref := range rel.References {
					if ref.OwnPrimaryKey {
						foreignFields = append(foreignFields, ref.PrimaryKey)
						relForeignKeys = append(relForeignKeys, ref.ForeignKey.DBName)
					} else if ref.PrimaryValue != "" {
						queryConds = append(queryConds, clause.Eq{
							Column: clause.Column{Table: rel.JoinTable.Table, Name: ref.ForeignKey.DBName},
							Value:  ref.PrimaryValue,
						})
					}
				}

				_, foreignValues := schema.GetIdentityFieldValuesMap(db.Statement.Context, db.Statement.ReflectValue, foreignFields)
				column, values := schema.ToQueryValues(table, relForeignKeys, foreignValues)
				queryConds = append(queryConds, clause.IN{Column: column, Values: values})

				// 删除关联数据
				if db.AddError(tx.Clauses(clause.Where{Exprs: queryConds}).Delete(modelValue).Error) != nil {
					return
				}
			}
		}

	}
}

func Delete(config *Config) func(db *gorm.DB) {
	supportReturning := utils.Contains(config.DeleteClauses, "RETURNING")

	return func(db *gorm.DB) {
		if db.Error != nil {
			return
		}

		// 数据库每张表就是一个 Schema
		if db.Statement.Schema != nil {
			for _, c := range db.Statement.Schema.DeleteClauses {
				db.Statement.AddClause(c)
			}
		}

		if db.Statement.SQL.Len() == 0 {
			db.Statement.SQL.Grow(100)
			db.Statement.AddClauseIfNotExists(clause.Delete{})

			if db.Statement.Schema != nil {
				_, queryValues := schema.GetIdentityFieldValuesMap(db.Statement.Context, db.Statement.ReflectValue, db.Statement.Schema.PrimaryFields)
				column, values := schema.ToQueryValues(db.Statement.Table, db.Statement.Schema.PrimaryFieldDBNames, queryValues)

				if len(values) > 0 {
					db.Statement.AddClause(clause.Where{Exprs: []clause.Expression{clause.IN{Column: column, Values: values}}})
				}

				if db.Statement.ReflectValue.CanAddr() && db.Statement.Dest != db.Statement.Model && db.Statement.Model != nil {
					_, queryValues = schema.GetIdentityFieldValuesMap(db.Statement.Context, reflect.ValueOf(db.Statement.Model), db.Statement.Schema.PrimaryFields)
					column, values = schema.ToQueryValues(db.Statement.Table, db.Statement.Schema.PrimaryFieldDBNames, queryValues)

					if len(values) > 0 {
						db.Statement.AddClause(clause.Where{Exprs: []clause.Expression{clause.IN{Column: column, Values: values}}})
					}
				}
			}

			db.Statement.AddClauseIfNotExists(clause.From{})

			db.Statement.Build(db.Statement.BuildClauses...)
		}

		checkMissingWhereConditions(db)

		if !db.DryRun && db.Error == nil {
			ok, mode := hasReturning(db, supportReturning)
			if !ok {
				result, err := db.Statement.ConnPool.ExecContext(db.Statement.Context, db.Statement.SQL.String(), db.Statement.Vars...)
				if db.AddError(err) == nil {
					db.RowsAffected, _ = result.RowsAffected()
				}

				return
			}

			if rows, err := db.Statement.ConnPool.QueryContext(db.Statement.Context, db.Statement.SQL.String(), db.Statement.Vars...); db.AddError(err) == nil {
				gorm.Scan(rows, db, mode)
				db.AddError(rows.Close())
			}
		}
	}
}

func AfterDelete(db *gorm.DB) {
	if db.Error == nil && db.Statement.Schema != nil && !db.Statement.SkipHooks && db.Statement.Schema.AfterDelete {
		callMethod(db, func(value interface{}, tx *gorm.DB) bool {
			if i, ok := value.(AfterDeleteInterface); ok {
				db.AddError(i.AfterDelete(tx))
				return true
			}
			return false
		})
	}
}
