package connections

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
)

const tableTransactionIdle = 10 * time.Minute

type tableTransaction struct {
	mu       sync.Mutex
	conn     *sql.Conn
	tx       *sql.Tx
	database string
	ctx      context.Context
	cancel   context.CancelFunc
	timer    *time.Timer
	done     bool
}

func (t *tableTransaction) finish(commit bool) error {
	if t.done {
		return nil
	}
	t.done = true
	if t.timer != nil {
		t.timer.Stop()
	}
	var err error
	if commit {
		err = t.tx.Commit()
	} else {
		err = t.tx.Rollback()
		if errors.Is(err, sql.ErrTxDone) {
			err = nil
		}
	}
	t.cancel()
	_ = t.conn.Raw(func(any) error { return driver.ErrBadConn })
	_ = t.conn.Close()
	return err
}
func (d *mysqlDatabase) takeTableTransaction(id string) *tableTransaction {
	d.transactionMu.Lock()
	defer d.transactionMu.Unlock()
	t := d.transactions[id]
	delete(d.transactions, id)
	return t
}
func (d *mysqlDatabase) expireTableTransaction(id string) {
	if t := d.takeTableTransaction(id); t != nil {
		t.cancel()
		t.mu.Lock()
		_ = t.finish(false)
		t.mu.Unlock()
	}
}
func (d *mysqlDatabase) closeTableTransactions() {
	d.transactionMu.Lock()
	d.transactionsClosed = true
	list := d.transactions
	d.transactions = nil
	d.transactionMu.Unlock()
	for _, t := range list {
		t.cancel()
	}
	for _, t := range list {
		t.mu.Lock()
		_ = t.finish(false)
		t.mu.Unlock()
	}
}
func (d *mysqlDatabase) beginTableTransaction(ctx context.Context, database, table string) (AdminResult, error) {
	result := AdminResult{Results: []QueryResult{}}
	if table == "" || len(table) > 256 || strings.ContainsRune(table, 0) {
		return result, errors.New("请选择有效数据表")
	}
	d.transactionMu.Lock()
	unavailable := d.transactionsClosed || len(d.transactions) >= 8
	d.transactionMu.Unlock()
	if unavailable {
		return result, errors.New("连接已关闭或已达到 8 个表事务上限")
	}
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return result, explainMySQL(err)
	}
	ready := false
	defer func() {
		if !ready {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			_ = conn.Close()
		}
	}()
	var engine, kind string
	if err = conn.QueryRowContext(ctx, "SELECT COALESCE(ENGINE,''),TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?", database, table).Scan(&engine, &kind); err != nil {
		return result, explainMySQL(err)
	}
	if kind != "BASE TABLE" || !strings.EqualFold(engine, "InnoDB") {
		return result, errors.New("当前表不是 InnoDB 基表，不能保证事务回滚")
	}
	if _, err = conn.ExecContext(ctx, "USE "+quoteIdentifier(database)); err != nil {
		return result, explainMySQL(err)
	}
	id, err := newID()
	if err != nil {
		return result, err
	}
	txctx, cancel := context.WithCancel(context.Background())
	stop := context.AfterFunc(ctx, cancel)
	tx, err := conn.BeginTx(txctx, nil)
	stop()
	if err != nil {
		cancel()
		return result, explainMySQL(err)
	}
	t := &tableTransaction{conn: conn, tx: tx, database: database, ctx: txctx, cancel: cancel}
	if ctx.Err() != nil {
		_ = t.finish(false)
		return result, ctx.Err()
	}
	d.transactionMu.Lock()
	if d.transactionsClosed || len(d.transactions) >= 8 {
		d.transactionMu.Unlock()
		_ = t.finish(false)
		return result, errors.New("连接已变化或打开的表事务过多")
	}
	if d.transactions == nil {
		d.transactions = map[string]*tableTransaction{}
	}
	t.timer = time.AfterFunc(tableTransactionIdle, func() { d.expireTableTransaction(id) })
	d.transactions[id] = t
	d.transactionMu.Unlock()
	ready = true
	result.TransactionID = id
	result.TransactionActive = true
	result.Completed = 1
	return result, nil
}

var transactionStatement = regexp.MustCompile(`(?i)^\s*(SELECT|SHOW|INSERT|UPDATE|DELETE)\b`)

func (d *mysqlDatabase) tableTransactionAdmin(ctx context.Context, database string, request AdminRequest) (AdminResult, error) {
	result := AdminResult{Results: []QueryResult{}}
	d.transactionMu.Lock()
	t := d.transactions[request.TransactionID]
	d.transactionMu.Unlock()
	if t == nil {
		if request.Operation == "transaction-rollback" {
			return result, nil
		}
		if request.Operation == "query" {
			result.Error = "事务已结束、断开或空闲超过 10 分钟，请刷新数据"
			return result, nil
		}
		return result, errors.New("事务已结束，无法确认提交状态，请刷新核对")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.database != database {
		return result, errors.New("事务数据库不匹配")
	}
	if t.done || t.ctx.Err() != nil {
		result.Error = "事务已结束，请刷新数据"
		return result, nil
	}
	t.timer.Stop()
	opctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(t.ctx, cancel)
	defer stop()
	switch request.Operation {
	case "transaction-commit", "transaction-rollback":
		d.takeTableTransaction(request.TransactionID)
		interrupt := context.AfterFunc(opctx, t.cancel)
		err := t.finish(request.Operation == "transaction-commit")
		interrupt()
		if err != nil {
			return result, errors.New("事务结束失败，提交状态可能不确定，请核对数据库：" + explainMySQL(err).Error())
		}
		result.Completed = 1
		return result, nil
	case "query":
		statements, err := SplitSQL(request.SQL)
		if err == nil && len(statements) == 0 {
			err = errors.New("请输入 SQL")
		}
		if err == nil {
			for _, statement := range statements {
				if !transactionStatement.MatchString(statement) {
					err = errors.New("表事务仅接受查询和行数据操作，禁止隐式提交的结构或会话命令")
					break
				}
			}
		}
		bytes := 0
		if err == nil {
			for _, statement := range statements {
				var q QueryResult
				q, err = queryResult(opctx, t.tx, statement, 1000)
				if err != nil {
					break
				}
				result.Completed++
				for _, row := range q.Rows {
					for _, value := range row {
						if value != nil {
							bytes += len(*value)
						}
					}
				}
				if bytes > 16<<20 {
					err = errors.New("事务查询结果累计超过 16 MB")
					break
				}
				if len(result.Results) < 100 {
					result.Results = append(result.Results, q)
				}
			}
		}
		if err != nil {
			d.takeTableTransaction(request.TransactionID)
			_ = t.finish(false)
			result.Error = "事务操作失败，已结束并请求回滚，请刷新核对：" + explainMySQL(err).Error()
			return result, nil
		}
		result.TransactionID = request.TransactionID
		result.TransactionActive = true
		t.timer.Reset(tableTransactionIdle)
		return result, nil
	default:
		t.timer.Reset(tableTransactionIdle)
		return result, errors.New("不支持的表事务操作")
	}
}
