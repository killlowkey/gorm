package clause

// Interface clause interface
type Interface interface {
	// Name clause name
	Name() string

	// Build 构建最终的 Clause SQL 语句，每个 Clause 都有自己的 Build 方法
	Build(Builder)

	// MergeClause 合并语句，比如多个 where 语句合并成一个
	// db.Model(&User{}).Where("name = ?", "jinzhu").Where("age = ?", 20)
	MergeClause(*Clause)
}

// ClauseBuilder clause builder, allows to customize how to build clause
type ClauseBuilder func(Clause, Builder)

type Writer interface {
	WriteByte(byte) error
	WriteString(string) (int, error)
}

// Builder builder interface
type Builder interface {
	Writer
	WriteQuoted(field interface{}) // 需要注意数据库方言
	AddVar(Writer, ...interface{})
	AddError(error) error
}

// Clause
type Clause struct {
	Name                string // WHERE
	BeforeExpression    Expression
	AfterNameExpression Expression
	AfterExpression     Expression
	Expression          Expression
	Builder             ClauseBuilder
}

// Build clause
func (c Clause) Build(builder Builder) {
	if c.Builder != nil {
		c.Builder(c, builder)
	} else if c.Expression != nil {
		if c.BeforeExpression != nil {
			c.BeforeExpression.Build(builder)
			builder.WriteByte(' ')
		}

		if c.Name != "" {
			builder.WriteString(c.Name)
			builder.WriteByte(' ')
		}

		if c.AfterNameExpression != nil {
			c.AfterNameExpression.Build(builder)
			builder.WriteByte(' ')
		}

		// 最终会调用 Statement 中的 WriteXXX 方法来构建 SQL 语句
		c.Expression.Build(builder)

		if c.AfterExpression != nil {
			builder.WriteByte(' ')
			c.AfterExpression.Build(builder)
		}
	}
}

const (
	PrimaryKey   string = "~~~py~~~" // primary key
	CurrentTable string = "~~~ct~~~" // current table
	Associations string = "~~~as~~~" // associations
)

var (
	currentTable  = Table{Name: CurrentTable}
	PrimaryColumn = Column{Table: CurrentTable, Name: PrimaryKey}
)

// Column quote with name
type Column struct {
	Table string // 表名
	Name  string // 字段名
	Alias string // 别名
	Raw   bool
}

// Table quote with name
type Table struct {
	Name  string
	Alias string
	Raw   bool
}
