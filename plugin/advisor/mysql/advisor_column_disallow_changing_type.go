package mysql

// Framework code is generated by the generator.

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb/parser/ast"

	"github.com/bytebase/bytebase/plugin/advisor"
	"github.com/bytebase/bytebase/plugin/advisor/catalog"
	"github.com/bytebase/bytebase/plugin/advisor/db"
)

var (
	_ advisor.Advisor = (*ColumnDisallowChangingTypeAdvisor)(nil)
	_ ast.Visitor     = (*columnDisallowChangingTypeChecker)(nil)
)

func init() {
	advisor.Register(db.MySQL, advisor.MySQLColumnDisallowChangingType, &ColumnDisallowChangingTypeAdvisor{})
	advisor.Register(db.TiDB, advisor.MySQLColumnDisallowChangingType, &ColumnDisallowChangingTypeAdvisor{})
}

// ColumnDisallowChangingTypeAdvisor is the advisor checking for disallow changing column type..
type ColumnDisallowChangingTypeAdvisor struct {
}

// Check checks for disallow changing column type..
func (*ColumnDisallowChangingTypeAdvisor) Check(ctx advisor.Context, statement string) ([]advisor.Advice, error) {
	stmtList, errAdvice := parseStatement(statement, ctx.Charset, ctx.Collation)
	if errAdvice != nil {
		return errAdvice, nil
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
	if err != nil {
		return nil, err
	}
	checker := &columnDisallowChangingTypeChecker{
		level:   level,
		title:   string(ctx.Rule.Type),
		catalog: ctx.Catalog,
	}

	for _, stmt := range stmtList {
		checker.text = stmt.Text()
		checker.line = stmt.OriginTextPosition()
		(stmt).Accept(checker)
	}

	if len(checker.adviceList) == 0 {
		checker.adviceList = append(checker.adviceList, advisor.Advice{
			Status:  advisor.Success,
			Code:    advisor.Ok,
			Title:   "OK",
			Content: "",
		})
	}
	return checker.adviceList, nil
}

type columnDisallowChangingTypeChecker struct {
	adviceList []advisor.Advice
	level      advisor.Status
	title      string
	text       string
	line       int
	catalog    *catalog.Finder
}

// Enter implements the ast.Visitor interface.
func (checker *columnDisallowChangingTypeChecker) Enter(in ast.Node) (ast.Node, bool) {
	changeType := false
	if node, ok := in.(*ast.AlterTableStmt); ok {
		for _, spec := range node.Specs {
			switch spec.Tp {
			case ast.AlterTableChangeColumn:
				changeType = checker.changeColumnType(node.Table.Name.O, spec.OldColumnName.Name.O, spec.NewColumns[0].Tp.String())
			case ast.AlterTableModifyColumn:
				changeType = checker.changeColumnType(node.Table.Name.O, spec.NewColumns[0].Name.Name.O, spec.NewColumns[0].Tp.String())
			}
			if changeType {
				break
			}
		}
	}

	if changeType {
		checker.adviceList = append(checker.adviceList, advisor.Advice{
			Status:  checker.level,
			Code:    advisor.ChangeColumnType,
			Title:   checker.title,
			Content: fmt.Sprintf("\"%s\" changes column type", checker.text),
			Line:    checker.line,
		})
	}

	return in, false
}

// Leave implements the ast.Visitor interface.
func (*columnDisallowChangingTypeChecker) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func normalizeColumnType(tp string) string {
	switch strings.ToLower(tp) {
	case "tinyint":
		return "tinyint(4)"
	case "tinyint unsigned":
		return "tinyint(4) unsigned"
	case "smallint":
		return "smallint(6)"
	case "smallint unsigned":
		return "smallint(6) unsigned"
	case "mediumint":
		return "mediumint(9)"
	case "mediumint unsigned":
		return "mediumint(9) unsigned"
	case "int":
		return "int(11)"
	case "int unsigned":
		return "int(11) unsigned"
	case "bigint":
		return "bigint(20)"
	case "bigint unsigned":
		return "bigint(20) unsigned"
	default:
		return strings.ToLower(tp)
	}
}

func (checker *columnDisallowChangingTypeChecker) changeColumnType(tableName string, columName string, newType string) bool {
	column := checker.catalog.Origin.FindColumn(&catalog.ColumnFind{
		TableName:  tableName,
		ColumnName: columName,
	})

	if column == nil {
		return false
	}

	return normalizeColumnType(column.Type()) != normalizeColumnType(newType)
}
