package clause

// Select select attrs when querying, updating, creating
// SELECT DISTINCT column1, column2 FROM your_table;
type Select struct {
	Distinct   bool       // 确保每一行都是唯一的
	Columns    []Column   // 选择的列，只能是单表中的字段，如果有连接操作，select 字段需要手动指定
	Expression Expression // 手动选择字段
}

func (s Select) Name() string {
	return "SELECT"
}

func (s Select) Build(builder Builder) {
	if len(s.Columns) > 0 {
		if s.Distinct {
			builder.WriteString("DISTINCT ")
		}

		for idx, column := range s.Columns {
			if idx > 0 {
				builder.WriteByte(',')
			}
			builder.WriteQuoted(column)
		}
	} else {
		// 未设置选择的列，则 select 所有
		builder.WriteByte('*')
	}
}

// MergeClause 手动添加的 select 语句
// select u.name, m.id from meeting m left join user u on u.id = m.u_id;
func (s Select) MergeClause(clause *Clause) {
	if s.Expression != nil {
		if s.Distinct {
			if expr, ok := s.Expression.(Expr); ok {
				expr.SQL = "DISTINCT " + expr.SQL
				clause.Expression = expr
				return
			}
		}

		clause.Expression = s.Expression
	} else {
		clause.Expression = s
	}
}

// CommaExpression represents a group of expressions separated by commas.
type CommaExpression struct {
	Exprs []Expression
}

func (comma CommaExpression) Build(builder Builder) {
	for idx, expr := range comma.Exprs {
		if idx > 0 {
			_, _ = builder.WriteString(", ")
		}
		expr.Build(builder)
	}
}
