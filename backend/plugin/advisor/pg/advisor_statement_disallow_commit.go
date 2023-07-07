package pg

// Framework code is generated by the generator.

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/plugin/advisor"
	"github.com/bytebase/bytebase/backend/plugin/advisor/db"
	"github.com/bytebase/bytebase/backend/plugin/parser/sql/ast"
)

var (
	_ advisor.Advisor = (*StatementDisallowCommitAdvisor)(nil)
	_ ast.Visitor     = (*statementDisallowCommitChecker)(nil)
)

func init() {
	advisor.Register(db.Postgres, advisor.PostgreSQLStatementDisallowCommit, &StatementDisallowCommitAdvisor{})
}

// StatementDisallowCommitAdvisor is the advisor checking for to disallow commit.
type StatementDisallowCommitAdvisor struct {
}

// Check checks for to disallow commit.
func (*StatementDisallowCommitAdvisor) Check(ctx advisor.Context, _ string) ([]advisor.Advice, error) {
	stmtList, ok := ctx.AST.([]ast.Node)
	if !ok {
		return nil, errors.Errorf("failed to convert to Node")
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
		ast.Walk(checker, stmt)
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
}

// Visit implements ast.Visitor interface.
func (checker *statementDisallowCommitChecker) Visit(in ast.Node) ast.Visitor {
	if _, ok := in.(*ast.CommitStmt); ok {
		checker.adviceList = append(checker.adviceList, advisor.Advice{
			Status:  checker.level,
			Code:    advisor.StatementDisallowCommit,
			Title:   checker.title,
			Content: fmt.Sprintf("Commit is not allowed, related statement: \"%s\"", in.Text()),
			Line:    in.LastLine(),
		})
	}

	return checker
}
