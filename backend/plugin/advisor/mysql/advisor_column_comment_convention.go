package mysql

// Framework code is generated by the generator.

import (
	"fmt"

	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/format"

	"github.com/bytebase/bytebase/backend/plugin/advisor"
	"github.com/bytebase/bytebase/backend/plugin/advisor/db"
)

var (
	_ advisor.Advisor = (*ColumnCommentConventionAdvisor)(nil)
	_ ast.Visitor     = (*columnCommentConventionChecker)(nil)
)

func init() {
	advisor.Register(db.MySQL, advisor.MySQLColumnCommentConvention, &ColumnCommentConventionAdvisor{})
	advisor.Register(db.TiDB, advisor.MySQLColumnCommentConvention, &ColumnCommentConventionAdvisor{})
	advisor.Register(db.MariaDB, advisor.MySQLColumnCommentConvention, &ColumnCommentConventionAdvisor{})
	advisor.Register(db.OceanBase, advisor.MySQLColumnCommentConvention, &ColumnCommentConventionAdvisor{})
}

// ColumnCommentConventionAdvisor is the advisor checking for column comment convention.
type ColumnCommentConventionAdvisor struct {
}

// Check checks for column comment convention.
func (*ColumnCommentConventionAdvisor) Check(ctx advisor.Context, statement string) ([]advisor.Advice, error) {
	stmtList, errAdvice := parseStatement(statement, ctx.Charset, ctx.Collation)
	if errAdvice != nil {
		return errAdvice, nil
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
	if err != nil {
		return nil, err
	}
	payload, err := advisor.UnmarshalCommentConventionRulePayload(ctx.Rule.Payload)
	if err != nil {
		return nil, err
	}
	checker := &columnCommentConventionChecker{
		level:     level,
		title:     string(ctx.Rule.Type),
		required:  payload.Required,
		maxLength: payload.MaxLength,
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

type columnCommentConventionChecker struct {
	adviceList []advisor.Advice
	level      advisor.Status
	title      string
	text       string
	line       int
	required   bool
	maxLength  int
}

type columnCommentData struct {
	exist   bool
	comment string
	table   string
	column  string
	line    int
}

// Enter implements the ast.Visitor interface.
func (checker *columnCommentConventionChecker) Enter(in ast.Node) (ast.Node, bool) {
	var columnList []columnCommentData
	switch node := in.(type) {
	case *ast.CreateTableStmt:
		for _, column := range node.Cols {
			exist, comment := checker.columnComment(column)
			columnList = append(columnList, columnCommentData{
				exist:   exist,
				comment: comment,
				table:   node.Table.Name.O,
				column:  column.Name.Name.O,
				line:    column.OriginTextPosition(),
			})
		}
	case *ast.AlterTableStmt:
		table := node.Table.Name.O
		for _, spec := range node.Specs {
			switch spec.Tp {
			case ast.AlterTableAddColumns:
				for _, column := range spec.NewColumns {
					exist, comment := checker.columnComment(column)
					columnList = append(columnList, columnCommentData{
						exist:   exist,
						comment: comment,
						table:   table,
						column:  column.Name.Name.O,
						line:    checker.line,
					})
				}
			case ast.AlterTableChangeColumn, ast.AlterTableModifyColumn:
				exist, comment := checker.columnComment(spec.NewColumns[0])
				columnList = append(columnList, columnCommentData{
					exist:   exist,
					comment: comment,
					table:   table,
					column:  spec.NewColumns[0].Name.Name.O,
					line:    checker.line,
				})
			}
		}
	}

	for _, column := range columnList {
		if checker.required && !column.exist {
			checker.adviceList = append(checker.adviceList, advisor.Advice{
				Status:  checker.level,
				Code:    advisor.NoColumnComment,
				Title:   checker.title,
				Content: fmt.Sprintf("Column `%s`.`%s` requires comments", column.table, column.column),
				Line:    column.line,
			})
		}
		if checker.maxLength >= 0 && len(column.comment) > checker.maxLength {
			checker.adviceList = append(checker.adviceList, advisor.Advice{
				Status:  checker.level,
				Code:    advisor.ColumnCommentTooLong,
				Title:   checker.title,
				Content: fmt.Sprintf("The length of column `%s`.`%s` comment should be within %d characters", column.table, column.column, checker.maxLength),
				Line:    column.line,
			})
		}
	}

	return in, false
}

// Leave implements the ast.Visitor interface.
func (*columnCommentConventionChecker) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func (checker *columnCommentConventionChecker) columnComment(column *ast.ColumnDef) (bool, string) {
	for _, option := range column.Options {
		if option.Tp == ast.ColumnOptionComment {
			comment, err := restoreNode(option.Expr, format.RestoreStringWithoutCharset)
			if err != nil {
				comment = ""
				checker.adviceList = append(checker.adviceList, advisor.Advice{
					Status:  checker.level,
					Code:    advisor.Internal,
					Title:   "Internal error for parsing column comment",
					Content: fmt.Sprintf("\"%q\" meet internal error %s", checker.text, err),
				})
			}
			return true, comment
		}
	}

	return false, ""
}
