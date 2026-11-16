package connections

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type objectBrowser interface {
	Objects(context.Context, string, string) (ObjectList, error)
}

// Objects reads only database metadata. Names remain bound values, never SQL identifiers.
func (d *mysqlDatabase) Objects(ctx context.Context, database, kind string) (ObjectList, error) {
	result := ObjectList{Objects: []DatabaseObject{}}
	var rows *sql.Rows
	var err error
	switch kind {
	case "tables", "views":
		tableType := "BASE TABLE"
		alternateType := "BASE TABLE"
		if kind == "views" {
			tableType = "VIEW"
			alternateType = "SYSTEM VIEW"
		}
		rows, err = d.db.QueryContext(ctx, `SELECT TABLE_NAME, COALESCE(ENGINE, ''), CAST(TABLE_ROWS AS CHAR),
			CAST(DATA_LENGTH AS CHAR), CAST(INDEX_LENGTH AS CHAR), CAST(AUTO_INCREMENT AS CHAR),
			COALESCE(TABLE_COMMENT, ''), CAST(CREATE_TIME AS CHAR), CAST(UPDATE_TIME AS CHAR)
			FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_TYPE IN (?, ?) ORDER BY TABLE_NAME LIMIT 10001`, database, tableType, alternateType)
	case "functions":
		rows, err = d.db.QueryContext(ctx, `SELECT ROUTINE_NAME, ROUTINE_TYPE, COALESCE(ROUTINE_COMMENT, ''),
			CAST(CREATED AS CHAR), CAST(LAST_ALTERED AS CHAR) FROM information_schema.ROUTINES
			WHERE ROUTINE_SCHEMA = ? ORDER BY ROUTINE_NAME LIMIT 10001`, database)
	case "events":
		rows, err = d.db.QueryContext(ctx, `SELECT EVENT_NAME, STATUS, COALESCE(EVENT_COMMENT, ''),
			CAST(CREATED AS CHAR), CAST(LAST_ALTERED AS CHAR) FROM information_schema.EVENTS
			WHERE EVENT_SCHEMA = ? ORDER BY EVENT_NAME LIMIT 10001`, database)
	default:
		return result, errors.New("暂不支持此对象分类")
	}
	if err != nil {
		return result, explainMySQL(err)
	}
	defer rows.Close()
	for rows.Next() {
		if len(result.Objects) >= 10000 {
			result.Truncated = true
			break
		}
		object := DatabaseObject{Kind: kind}
		var created, modified, count, data, index, increment sql.NullString
		if kind == "tables" || kind == "views" {
			err = rows.Scan(&object.Name, &object.Engine, &count, &data, &index, &increment, &object.Comment, &created, &modified)
			object.Rows, object.DataLength, object.IndexLength, object.AutoIncrement = count.String, data.String, index.String, increment.String
		} else {
			err = rows.Scan(&object.Name, &object.Engine, &object.Comment, &created, &modified)
		}
		if err != nil {
			return result, errors.New("无法读取数据库对象的元数据")
		}
		object.CreatedAt, object.ModifiedAt = created.String, modified.String
		result.Objects = append(result.Objects, object)
	}
	if err = rows.Err(); err != nil {
		return result, explainMySQL(err)
	}
	return result, nil
}

func (s *Service) Objects(id, database, kind string) (ObjectList, error) {
	if database == "" || len(database) > 256 || strings.ContainsRune(database, '\x00') {
		return ObjectList{}, errors.New("请选择有效的数据库")
	}
	if kind != "tables" && kind != "views" && kind != "functions" && kind != "events" {
		return ObjectList{}, errors.New("暂不支持此对象分类")
	}
	s.mu.Lock()
	live, ok := s.sessions[id]
	epoch := s.epoch[id]
	s.mu.Unlock()
	if !ok {
		return ObjectList{}, errors.New("请先打开连接")
	}
	allowed := false
	for _, name := range live.info.Databases {
		if name == database {
			allowed = true
			break
		}
	}
	if !allowed {
		return ObjectList{}, errors.New("数据库不在当前账户可访问的列表中，请刷新连接")
	}
	browser, ok := live.db.(objectBrowser)
	if !ok {
		return ObjectList{}, errors.New("此连接不支持浏览数据库对象")
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(live.timeout)*time.Second)
	defer cancel()
	result, err := browser.Objects(ctx, database, kind)
	s.mu.Lock()
	current := s.epoch[id]
	s.mu.Unlock()
	if current != epoch {
		return ObjectList{}, errors.New("连接已断开或配置已变化，请重新选择数据库")
	}
	return result, err
}
