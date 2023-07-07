// Package snowflake is the advisor for snowflake database.
package snowflake

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/snowsql-parser"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/plugin/advisor"
	"github.com/bytebase/bytebase/backend/plugin/advisor/db"
	snowsqlparser "github.com/bytebase/bytebase/backend/plugin/parser/sql"
)

var (
	_ advisor.Advisor = (*TableRequirePkAdvisor)(nil)
)

func init() {
	advisor.Register(db.Snowflake, advisor.SnowflakeTableRequirePK, &TableRequirePkAdvisor{})
}

// TableRequirePkAdvisor is the advisor checking for table require primary key.
type TableRequirePkAdvisor struct {
}

// Check checks for table require primary key.
func (*TableRequirePkAdvisor) Check(ctx advisor.Context, _ string) ([]advisor.Advice, error) {
	tree, ok := ctx.AST.(antlr.Tree)
	if !ok {
		return nil, errors.Errorf("failed to convert to Tree")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
	if err != nil {
		return nil, err
	}

	listener := &tableRequirePkChecker{
		level:                      level,
		title:                      string(ctx.Rule.Type),
		currentConstraintAction:    currentConstraintActionNone,
		currentNormalizedTableName: "",
		tableHasPrimaryKey:         make(map[string]bool),
		tableOriginalName:          make(map[string]string),
		tableLine:                  make(map[string]int),
	}

	antlr.ParseTreeWalkerDefault.Walk(listener, tree)

	return listener.generateAdvice()
}

// tableRequirePkChecker is the listener for table require primary key.
type tableRequirePkChecker struct {
	*parser.BaseSnowflakeParserListener

	level advisor.Status
	title string

	adviceList []advisor.Advice

	currentConstraintAction currentConstraintAction
	// currentNormalizedTableName is the current table name, and it is normalized.
	// It should be set then entering create_table, alter_table and so on,
	// and should be reset then exiting them.
	currentNormalizedTableName string

	// tableHasPrimaryKey is a map of normalized table name to whether the table has primary key.
	tableHasPrimaryKey map[string]bool
	// tableOriginalName is a map of normalized table name to original table name.
	// The key of the tableOriginalName is the superset of the key of the tableHasPrimaryKey.
	tableOriginalName map[string]string
	// tableLine is a map of normalized table name to the line number of the table.
	// The key of the tableLine is the superset of the key of the tableHasPrimaryKey.
	tableLine map[string]int
}

// generateAdvice returns the advices generated by the listener, the advices must not be empty.
func (l *tableRequirePkChecker) generateAdvice() ([]advisor.Advice, error) {
	for tableName, has := range l.tableHasPrimaryKey {
		if !has {
			l.adviceList = append(l.adviceList, advisor.Advice{
				Status:  l.level,
				Code:    advisor.TableNoPK,
				Title:   l.title,
				Content: fmt.Sprintf("Table %s requires PRIMARY KEY.", l.tableOriginalName[tableName]),
				Line:    l.tableLine[tableName],
			})
		}
	}
	if len(l.adviceList) == 0 {
		return []advisor.Advice{
			{
				Status:  advisor.Success,
				Code:    advisor.Ok,
				Title:   "OK",
				Content: "",
			},
		}, nil
	}
	return l.adviceList, nil
}

// EnterCreate_table is called when production create_table is entered.
func (l *tableRequirePkChecker) EnterCreate_table(ctx *parser.Create_tableContext) {
	originalTableName := ctx.Object_name()
	normalizedTableName := snowsqlparser.NormalizeObjectName(originalTableName)

	l.tableHasPrimaryKey[normalizedTableName] = false
	l.tableOriginalName[normalizedTableName] = originalTableName.GetText()
	l.tableLine[normalizedTableName] = ctx.GetStart().GetLine()
	l.currentNormalizedTableName = normalizedTableName
	l.currentConstraintAction = currentConstraintActionAdd
}

// EnterDrop_table is called when production drop_table is entered.
func (l *tableRequirePkChecker) EnterDrop_table(ctx *parser.Drop_tableContext) {
	originalTableName := ctx.Object_name()
	normalizedTableName := snowsqlparser.NormalizeObjectName(originalTableName)

	delete(l.tableHasPrimaryKey, normalizedTableName)
	delete(l.tableOriginalName, normalizedTableName)
	delete(l.tableLine, normalizedTableName)
}

// ExitCreate_table is called when production create_table is exited.
func (l *tableRequirePkChecker) ExitCreate_table(*parser.Create_tableContext) {
	l.currentNormalizedTableName = ""
	l.currentConstraintAction = currentConstraintActionNone
}

// EnterInline_constraint is called when production inline_constraint is entered.
func (l *tableRequirePkChecker) EnterInline_constraint(ctx *parser.Inline_constraintContext) {
	if ctx.PRIMARY() == nil || l.currentNormalizedTableName == "" {
		return
	}
	l.tableHasPrimaryKey[l.currentNormalizedTableName] = true
}

// EnterOut_of_line_constraint is called when production out_of_line_constraint is entered.
func (l *tableRequirePkChecker) EnterOut_of_line_constraint(ctx *parser.Out_of_line_constraintContext) {
	if ctx.PRIMARY() == nil || l.currentNormalizedTableName == "" || l.currentConstraintAction == currentConstraintActionNone {
		return
	}
	if l.currentConstraintAction == currentConstraintActionAdd {
		l.tableHasPrimaryKey[l.currentNormalizedTableName] = true
	} else if l.currentConstraintAction == currentConstraintActionDrop {
		l.tableHasPrimaryKey[l.currentNormalizedTableName] = false
		l.tableLine[l.currentNormalizedTableName] = ctx.GetStart().GetLine()
	}
}

// EnterConstraint_action is called when production constraint_action is entered.
func (l *tableRequirePkChecker) EnterConstraint_action(ctx *parser.Constraint_actionContext) {
	if l.currentNormalizedTableName == "" {
		return
	}
	if ctx.DROP() != nil && ctx.PRIMARY() != nil {
		if _, ok := l.tableHasPrimaryKey[l.currentNormalizedTableName]; ok {
			l.tableHasPrimaryKey[l.currentNormalizedTableName] = false
			l.tableLine[l.currentNormalizedTableName] = ctx.GetStart().GetLine()
		}
		return
	}
	if ctx.ADD() != nil {
		l.currentConstraintAction = currentConstraintActionAdd
		return
	}
}

// EnterAlter_table is called when production alter_table is entered.
func (l *tableRequirePkChecker) EnterAlter_table(ctx *parser.Alter_tableContext) {
	if ctx.Constraint_action() == nil {
		return
	}
	originalTableName := ctx.Object_name(0)
	normalizedTableName := snowsqlparser.NormalizeObjectName(originalTableName)

	l.currentNormalizedTableName = normalizedTableName
	l.tableOriginalName[normalizedTableName] = originalTableName.GetText()
}

// ExitAlter_table is called when production alter_table is exited.
func (l *tableRequirePkChecker) ExitAlter_table(*parser.Alter_tableContext) {
	l.currentNormalizedTableName = ""
	l.currentConstraintAction = currentConstraintActionNone
}
