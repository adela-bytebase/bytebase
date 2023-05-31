package mysql

// Framework code is generated by the generator.

import (
	"fmt"

	"github.com/pingcap/tidb/parser/ast"

	"github.com/bytebase/bytebase/backend/plugin/advisor"
	"github.com/bytebase/bytebase/backend/plugin/advisor/db"
)

var (
	_ advisor.Advisor = (*StatementDisallowCommitAdvisor)(nil)
	_ ast.Visitor     = (*statementDisallowCommitChecker)(nil)
)

func init() {
	advisor.Register(db.MySQL, advisor.MySQLStatementDisallowCommit, &StatementDisallowCommitAdvisor{})
	advisor.Register(db.TiDB, advisor.MySQLStatementDisallowCommit, &StatementDisallowCommitAdvisor{})
	advisor.Register(db.MariaDB, advisor.MySQLStatementDisallowCommit, &StatementDisallowCommitAdvisor{})
	advisor.Register(db.OceanBase, advisor.MySQLStatementDisallowCommit, &StatementDisallowCommitAdvisor{})
}

// StatementDisallowCommitAdvisor is the advisor checking for index type no blob.
type StatementDisallowCommitAdvisor struct {
}

// Check checks for index type no blob.
func (*StatementDisallowCommitAdvisor) Check(ctx advisor.Context, statement string) ([]advisor.Advice, error) {
	stmtList, errAdvice := parseStatement(statement, ctx.Charset, ctx.Collation)
	if errAdvice != nil {
		return errAdvice, nil
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
	if err != nil {
		return nil, err
	}
	checker := &statementDisallowCommitChecker{
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

type statementDisallowCommitChecker struct {
	adviceList []advisor.Advice
	level      advisor.Status
	title      string
	text       string
	line       int
}

// Enter implements the ast.Visitor interface.
func (c *statementDisallowCommitChecker) Enter(in ast.Node) (ast.Node, bool) {
	if _, ok := in.(*ast.CommitStmt); ok {
		c.adviceList = append(c.adviceList, advisor.Advice{
			Status:  c.level,
			Code:    advisor.StatementDisallowCommit,
			Title:   c.title,
			Content: fmt.Sprintf("Commit is not allowed, related statement: \"%s\"", c.text),
			Line:    c.line,
		})
	}

	return in, false
}

// Leave implements the ast.Visitor interface.
func (*statementDisallowCommitChecker) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}
