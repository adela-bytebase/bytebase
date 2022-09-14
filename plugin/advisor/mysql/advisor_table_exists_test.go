package mysql

// Framework code is generated by the generator.

import (
	"testing"

	"github.com/bytebase/bytebase/plugin/advisor"
)

func TestTableExists(t *testing.T) {
	tests := []advisor.TestCase{
		{
			Statement: `INSERT INTO tech_book values (1)`,
			Want: []advisor.Advice{
				{
					Status:  advisor.Success,
					Code:    advisor.Ok,
					Title:   "OK",
					Content: "",
				},
			},
		},
		{
			Statement: `INSERT INTO t values (1)`,
			Want: []advisor.Advice{
				{
					Status:  advisor.Warn,
					Code:    advisor.TableNotExists,
					Title:   "table.exists",
					Content: "Table `t` not exists, related statement: \"INSERT INTO t values (1)\"",
					Line:    1,
				},
			},
		},
		{
			Statement: `ALTER TABLE t ADD COLUMN a int`,
			Want: []advisor.Advice{
				{
					Status:  advisor.Warn,
					Code:    advisor.TableNotExists,
					Title:   "table.exists",
					Content: "Table `t` not exists, related statement: \"ALTER TABLE t ADD COLUMN a int\"",
					Line:    1,
				},
			},
		},
		{
			Statement: `CREATE TABLE t_copy LIKE t`,
			Want: []advisor.Advice{
				{
					Status:  advisor.Warn,
					Code:    advisor.TableNotExists,
					Title:   "table.exists",
					Content: "Table `t` not exists, related statement: \"CREATE TABLE t_copy LIKE t\"",
					Line:    1,
				},
			},
		},
		{
			Statement: `ALTER TABLE tech_book RENAME TO tech`,
			Want: []advisor.Advice{
				{
					Status:  advisor.Success,
					Code:    advisor.Ok,
					Title:   "OK",
					Content: "",
				},
			},
		},
		{
			Statement: `CREATE TABLE tech_book_copy LIKE tech_book`,
			Want: []advisor.Advice{
				{
					Status:  advisor.Success,
					Code:    advisor.Ok,
					Title:   "OK",
					Content: "",
				},
			},
		},
		{
			Statement: `CREATE TABLE tech_book_copy AS SELECT * from t`,
			Want: []advisor.Advice{
				{
					Status:  advisor.Success,
					Code:    advisor.Ok,
					Title:   "OK",
					Content: "",
				},
			},
		},
		{
			Statement: `CREATE TABLE tech_book_copy (a int)`,
			Want: []advisor.Advice{
				{
					Status:  advisor.Success,
					Code:    advisor.Ok,
					Title:   "OK",
					Content: "",
				},
			},
		},
	}

	advisor.RunSQLReviewRuleTests(t, tests, &TableExistsAdvisor{}, &advisor.SQLReviewRule{
		Type:    advisor.SchemaRuleTableExists,
		Level:   advisor.SchemaRuleLevelWarning,
		Payload: "",
	}, advisor.MockMySQLDatabase)
}
