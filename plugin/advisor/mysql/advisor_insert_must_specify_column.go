package mysql

// Framework code is generated by the generator.

import (
	"fmt"

	"github.com/pingcap/tidb/parser/ast"

	"github.com/bytebase/bytebase/plugin/advisor"
	"github.com/bytebase/bytebase/plugin/advisor/db"
)

var (
	_ advisor.Advisor = (*InsertMustSpecifyColumnAdvisor)(nil)
	_ ast.Visitor     = (*insertMustSpecifyColumnChecker)(nil)
)

func init() {
	advisor.Register(db.MySQL, advisor.MySQLInsertMustSpecifyColumn, &InsertMustSpecifyColumnAdvisor{})
	advisor.Register(db.TiDB, advisor.MySQLInsertMustSpecifyColumn, &InsertMustSpecifyColumnAdvisor{})
}

// InsertMustSpecifyColumnAdvisor is the advisor checking for to enforce column specified.
type InsertMustSpecifyColumnAdvisor struct {
}

// Check checks for to enforce column specified.
func (*InsertMustSpecifyColumnAdvisor) Check(ctx advisor.Context, statement string) ([]advisor.Advice, error) {
	stmtList, errAdvice := parseStatement(statement, ctx.Charset, ctx.Collation)
	if errAdvice != nil {
		return errAdvice, nil
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
	if err != nil {
		return nil, err
	}
	checker := &insertMustSpecifyColumnChecker{
		level: level,
		title: string(ctx.Rule.Type),
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

type insertMustSpecifyColumnChecker struct {
	adviceList []advisor.Advice
	level      advisor.Status
	title      string
	text       string
	line       int
}

// Enter implements the ast.Visitor interface.
func (checker *insertMustSpecifyColumnChecker) Enter(in ast.Node) (ast.Node, bool) {
	if node, ok := in.(*ast.InsertStmt); ok {
		if node.Columns == nil {
			checker.adviceList = append(checker.adviceList, advisor.Advice{
				Status:  checker.level,
				Code:    advisor.InsertNotSpecifyColumn,
				Title:   checker.title,
				Content: fmt.Sprintf("The INSERT statement must specify columns but \"%s\" does not", checker.text),
				Line:    checker.line,
			})
		}
	}

	return in, false
}

// Leave implements the ast.Visitor interface.
func (*insertMustSpecifyColumnChecker) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}
