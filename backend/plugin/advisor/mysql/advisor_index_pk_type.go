package mysql

// Framework code is generated by the generator.

import (
	"fmt"

	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/parser/mysql"
	"github.com/pingcap/tidb/parser/types"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/plugin/advisor"
	"github.com/bytebase/bytebase/backend/plugin/advisor/catalog"
	"github.com/bytebase/bytebase/backend/plugin/advisor/db"
)

var (
	_ advisor.Advisor = (*IndexPkTypeAdvisor)(nil)
	_ ast.Visitor     = (*indexPkTypeChecker)(nil)
)

func init() {
	advisor.Register(db.MySQL, advisor.MySQLIndexPKType, &IndexPkTypeAdvisor{})
	advisor.Register(db.TiDB, advisor.MySQLIndexPKType, &IndexPkTypeAdvisor{})
	advisor.Register(db.MariaDB, advisor.MySQLIndexPKType, &IndexPkTypeAdvisor{})
	advisor.Register(db.OceanBase, advisor.MySQLIndexPKType, &IndexPkTypeAdvisor{})
}

// IndexPkTypeAdvisor is the advisor checking for correct type of PK.
type IndexPkTypeAdvisor struct {
}

// Check checks for correct type of PK.
func (*IndexPkTypeAdvisor) Check(ctx advisor.Context, statement string) ([]advisor.Advice, error) {
	stmtList, errAdvice := parseStatement(statement, ctx.Charset, ctx.Collation)
	if errAdvice != nil {
		return errAdvice, nil
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
	if err != nil {
		return nil, err
	}
	checker := &indexPkTypeChecker{
		level:            level,
		title:            string(ctx.Rule.Type),
		line:             make(map[string]int),
		catalog:          ctx.Catalog,
		tablesNewColumns: make(map[string]columnNameToColumnDef),
	}

	for _, stmt := range stmtList {
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

type columnNameToColumnDef map[string]*ast.ColumnDef
type tableNewColumn map[string]columnNameToColumnDef

func (t tableNewColumn) set(tableName string, columnName string, colDef *ast.ColumnDef) {
	if _, ok := t[tableName]; !ok {
		t[tableName] = make(map[string]*ast.ColumnDef)
	}
	t[tableName][columnName] = colDef
}

func (t tableNewColumn) get(tableName string, columnName string) (colDef *ast.ColumnDef, ok bool) {
	if _, ok := t[tableName]; !ok {
		return nil, false
	}
	col, ok := t[tableName][columnName]
	return col, ok
}

func (t tableNewColumn) delete(tableName string, columnName string) {
	if _, ok := t[tableName]; !ok {
		return
	}
	delete(t[tableName], columnName)
}

type indexPkTypeChecker struct {
	adviceList       []advisor.Advice
	level            advisor.Status
	title            string
	line             map[string]int
	catalog          *catalog.Finder
	tablesNewColumns tableNewColumn
}

type pkData struct {
	table      string
	column     string
	columnType string
	line       int
}

// Enter implements the ast.Visitor interface.
func (v *indexPkTypeChecker) Enter(in ast.Node) (ast.Node, bool) {
	var pkDataList []pkData
	switch node := in.(type) {
	case *ast.CreateTableStmt:
		tableName := node.Table.Name.String()
		for _, column := range node.Cols {
			pds := v.addNewColumn(tableName, column.OriginTextPosition(), column)
			pkDataList = append(pkDataList, pds...)
		}
		for _, constraint := range node.Constraints {
			pds := v.addConstraint(tableName, constraint.OriginTextPosition(), constraint)
			pkDataList = append(pkDataList, pds...)
		}
	case *ast.AlterTableStmt:
		tableName := node.Table.Name.String()
		for _, spec := range node.Specs {
			switch spec.Tp {
			case ast.AlterTableAddColumns:
				for _, column := range spec.NewColumns {
					pds := v.addNewColumn(tableName, node.OriginTextPosition(), column)
					pkDataList = append(pkDataList, pds...)
				}
			case ast.AlterTableAddConstraint:
				pds := v.addConstraint(tableName, node.OriginTextPosition(), spec.Constraint)
				pkDataList = append(pkDataList, pds...)
			case ast.AlterTableChangeColumn, ast.AlterTableModifyColumn:
				newColumnDef := spec.NewColumns[0]
				oldColumnName := newColumnDef.Name.Name.String()
				if spec.OldColumnName != nil {
					oldColumnName = spec.OldColumnName.Name.String()
				}
				pds := v.changeColumn(tableName, oldColumnName, node.OriginTextPosition(), newColumnDef)
				pkDataList = append(pkDataList, pds...)
			}
		}
	}
	for _, pd := range pkDataList {
		v.adviceList = append(v.adviceList, advisor.Advice{
			Status:  v.level,
			Code:    advisor.IndexPKType,
			Title:   v.title,
			Content: fmt.Sprintf("Columns in primary key must be INT/BIGINT but `%s`.`%s` is %s", pd.table, pd.column, pd.columnType),
			Line:    pd.line,
		})
	}
	return in, false
}

// Leave implements the ast.Visitor interface.
func (*indexPkTypeChecker) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func (v *indexPkTypeChecker) addNewColumn(tableName string, line int, colDef *ast.ColumnDef) []pkData {
	var pkDataList []pkData
	for _, option := range colDef.Options {
		if option.Tp == ast.ColumnOptionPrimaryKey {
			tp := v.getIntOrBigIntStr(colDef.Tp)
			if tp != "INT" && tp != "BIGINT" {
				pkDataList = append(pkDataList, pkData{
					table:      tableName,
					column:     colDef.Name.String(),
					columnType: tp,
					line:       line,
				})
			}
		}
	}
	v.tablesNewColumns.set(tableName, colDef.Name.String(), colDef)
	return pkDataList
}

func (v *indexPkTypeChecker) changeColumn(tableName, oldColumnName string, line int, newColumnDef *ast.ColumnDef) []pkData {
	var pkDataList []pkData
	v.tablesNewColumns.delete(tableName, oldColumnName)
	for _, option := range newColumnDef.Options {
		if option.Tp == ast.ColumnOptionPrimaryKey {
			tp := v.getIntOrBigIntStr(newColumnDef.Tp)
			if tp != "INT" && tp != "BIGINT" {
				pkDataList = append(pkDataList, pkData{
					table:      tableName,
					column:     newColumnDef.Name.String(),
					columnType: tp,
					line:       line,
				})
			}
		}
	}
	v.tablesNewColumns.set(tableName, newColumnDef.Name.String(), newColumnDef)
	return pkDataList
}

func (v *indexPkTypeChecker) addConstraint(tableName string, line int, constraint *ast.Constraint) []pkData {
	var pkDataList []pkData
	if constraint.Tp == ast.ConstraintPrimaryKey {
		for _, key := range constraint.Keys {
			columnName := key.Column.Name.String()
			columnType, err := v.getPKColumnType(tableName, columnName)
			if err != nil {
				continue
			}
			if columnType != "INT" && columnType != "BIGINT" {
				pkDataList = append(pkDataList, pkData{
					table:      tableName,
					column:     columnName,
					columnType: columnType,
					line:       line,
				})
			}
		}
	}
	return pkDataList
}

// getPKColumnType gets the column type string from v.tablesNewColumns or catalog, returns empty string and non-nil error if cannot find the column in given table.
func (v *indexPkTypeChecker) getPKColumnType(tableName string, columnName string) (string, error) {
	if colDef, ok := v.tablesNewColumns.get(tableName, columnName); ok {
		return v.getIntOrBigIntStr(colDef.Tp), nil
	}
	column := v.catalog.Origin.FindColumn(&catalog.ColumnFind{
		TableName:  tableName,
		ColumnName: columnName,
	})
	if column != nil {
		return column.Type(), nil
	}
	return "", errors.Errorf("cannot find the type of `%s`.`%s`", tableName, columnName)
}

// getIntOrBigIntStr returns the type string of tp.
func (*indexPkTypeChecker) getIntOrBigIntStr(tp *types.FieldType) string {
	switch tp.GetType() {
	// https://pkg.go.dev/github.com/pingcap/tidb/parser/mysql#TypeLong
	case mysql.TypeLong:
		// tp.String() return int(11)
		return "INT"
		// https://pkg.go.dev/github.com/pingcap/tidb/parser/mysql#TypeLonglong
	case mysql.TypeLonglong:
		// tp.String() return bigint(20)
		return "BIGINT"
	}
	return tp.String()
}