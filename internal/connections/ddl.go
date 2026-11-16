package connections

import (
	"context"
	"errors"
	"strings"
	"time"
)

type ddlBrowser interface {
	TableDDL(context.Context, string, string) (string, error)
}

func quoteIdentifier(name string) string { return "`" + strings.ReplaceAll(name, "`", "``") + "`" }
func (d *mysqlDatabase) TableDDL(ctx context.Context, database, table string) (string, error) {
	var count int
	if err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND TABLE_TYPE='BASE TABLE'", database, table).Scan(&count); err != nil {
		return "", explainMySQL(err)
	}
	if count != 1 {
		return "", errors.New("此表不存在或当前账户不可访问")
	}
	var name, ddl string
	if err := d.db.QueryRowContext(ctx, "SHOW CREATE TABLE "+quoteIdentifier(database)+"."+quoteIdentifier(table)).Scan(&name, &ddl); err != nil {
		return "", explainMySQL(err)
	}
	if len(ddl) > 1<<20 {
		return "", errors.New("表定义超过 1 MB")
	}
	return ddl, nil
}
func (s *Service) TableDDL(id, database, table string) (string, error) {
	if database == "" || table == "" || len(database) > 256 || len(table) > 256 || strings.ContainsAny(database+table, "\x00") {
		return "", errors.New("请选择有效的数据库和表")
	}
	s.mu.Lock()
	live, ok := s.sessions[id]
	epoch := s.epoch[id]
	s.mu.Unlock()
	if !ok {
		return "", errors.New("请先打开连接")
	}
	allowed := false
	for _, name := range live.info.Databases {
		if name == database {
			allowed = true
		}
	}
	if !allowed {
		return "", errors.New("数据库不在当前账户可访问的列表中")
	}
	browser, ok := live.db.(ddlBrowser)
	if !ok {
		return "", errors.New("此连接不支持查看表定义")
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(live.timeout)*time.Second)
	defer cancel()
	ddl, err := browser.TableDDL(ctx, database, table)
	s.mu.Lock()
	current := s.epoch[id]
	s.mu.Unlock()
	if current != epoch {
		return "", errors.New("连接已变化，请重新打开表")
	}
	return ddl, err
}
