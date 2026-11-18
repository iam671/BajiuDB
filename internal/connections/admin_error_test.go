package connections

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestAdminErrorCode(t *testing.T) {
	if code := adminErrorCode(fmt.Errorf("statement: %w", &mysql.MySQLError{Number: 1050, Message: "table exists"})); code != 1050 {
		t.Fatalf("expected structured duplicate-table code, got %d", code)
	}
	if code := adminErrorCode(errors.New("network failure")); code != 0 {
		t.Fatalf("non-server errors must not invent a MySQL code: %d", code)
	}
}
