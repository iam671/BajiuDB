package connections

import "testing"

func TestSplitSQLForDesignerMode(t *testing.T) {
	cases := []struct {
		sql, mode string
		count     int
	}{
		{`SELECT 'trailing\'; SELECT 2;`, "NO_BACKSLASH_ESCAPES", 2},
		{`SELECT 'quote '' and \ path'; SELECT 2;`, "STRICT_TRANS_TABLES,NO_BACKSLASH_ESCAPES", 2},
		{`SELECT 'escaped \' quote; text'; SELECT 2;`, "", 2},
		{"DELIMITER $$\nCREATE TRIGGER t BEFORE INSERT ON x FOR EACH ROW BEGIN SET NEW.v='tail\\'; SET NEW.n=1; END$$\nDELIMITER ;\nSELECT 2;", "NO_BACKSLASH_ESCAPES", 2},
		{`SELECT "column\" FROM x; SELECT 2;`, "ANSI_QUOTES", 2},
	}
	for _, tc := range cases {
		got, err := SplitSQLForMode(tc.sql, tc.mode)
		if err != nil || len(got) != tc.count {
			t.Fatalf("mode %q: %v, %#v", tc.mode, err, got)
		}
	}
	if _, err := SplitSQLForMode(`SELECT 'trailing\'; SELECT 2;`, ""); err == nil {
		t.Fatal("normal mode accepted an unterminated escaped string")
	}
}
