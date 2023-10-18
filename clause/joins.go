package clause

type JoinType string

const (
	CrossJoin JoinType = "CROSS"
	InnerJoin JoinType = "INNER"
	LeftJoin  JoinType = "LEFT"
	RightJoin JoinType = "RIGHT"
)

// Join clause for from
// select * from a left join b on a.m_id = b.m_id;
// select * from a left join b using m_id;
type Join struct {
	Type       JoinType   // 表连接类型
	Table      Table      // 连接的表
	ON         Where      // 连接条件
	Using      []string   // a表和b表有相同字段，可以作为连接条件 using m_id
	Expression Expression // 自定义 join，参考 joins_test.go
}

func (join Join) Build(builder Builder) {
	if join.Expression != nil {
		join.Expression.Build(builder)
	} else {
		if join.Type != "" {
			builder.WriteString(string(join.Type))
			builder.WriteByte(' ')
		}

		builder.WriteString("JOIN ")
		builder.WriteQuoted(join.Table)

		if len(join.ON.Exprs) > 0 {
			builder.WriteString(" ON ")
			join.ON.Build(builder)
		} else if len(join.Using) > 0 {
			builder.WriteString(" USING (")
			for idx, c := range join.Using {
				if idx > 0 {
					builder.WriteByte(',')
				}
				builder.WriteQuoted(c)
			}
			builder.WriteByte(')')
		}
	}
}
