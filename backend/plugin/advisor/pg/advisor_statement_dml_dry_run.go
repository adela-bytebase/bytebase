package pg

// Framework code is generated by the generator.

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bytebase/bytebase/backend/plugin/advisor"
	"github.com/bytebase/bytebase/backend/plugin/advisor/db"
	"github.com/bytebase/bytebase/backend/plugin/parser/sql/ast"
)

var (
	_ advisor.Advisor = (*StatementDmlDryRunAdvisor)(nil)
	_ ast.Visitor     = (*statementDmlDryRunChecker)(nil)
)

func init() {
	advisor.Register(db.Postgres, advisor.PostgreSQLStatementDMLDryRun, &StatementDmlDryRunAdvisor{})
}

// StatementDmlDryRunAdvisor is the advisor checking for DML dry run.
type StatementDmlDryRunAdvisor struct {
}

// Check checks for DML dry run.
func (*StatementDmlDryRunAdvisor) Check(ctx advisor.Context, statement string) ([]advisor.Advice, error) {
	stmtList, errAdvice := parseStatement(statement)
	if errAdvice != nil {
		return errAdvice, nil
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
	if err != nil {
		return nil, err
	}
	checker := &statementDmlDryRunChecker{
		level:  level,
		title:  string(ctx.Rule.Type),
		driver: ctx.Driver,
		ctx:    ctx.Context,
	}

	if checker.driver != nil {
		for _, stmt := range stmtList {
			ast.Walk(checker, stmt)
		}
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

type statementDmlDryRunChecker struct {
	adviceList []advisor.Advice
	level      advisor.Status
	title      string
	driver     *sql.DB
	ctx        context.Context
}

// Visit implements ast.Visitor interface.
func (checker *statementDmlDryRunChecker) Visit(in ast.Node) ast.Visitor {
	switch node := in.(type) {
	case *ast.InsertStmt, *ast.UpdateStmt, *ast.DeleteStmt:
		if _, err := advisor.Query(checker.ctx, checker.driver, fmt.Sprintf("EXPLAIN %s", node.Text())); err != nil {
			checker.adviceList = append(checker.adviceList, advisor.Advice{
				Status:  checker.level,
				Code:    advisor.StatementDMLDryRunFailed,
				Title:   checker.title,
				Content: fmt.Sprintf("\"%s\" dry runs failed: %s", node.Text(), err.Error()),
				Line:    node.LastLine(),
			})
		}
	}

	return checker
}
