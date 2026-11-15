package connections

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Repository interface {
	Load() ([]Record, error)
	Save([]Record) error
}

type FileStore struct{ Path string }
type document struct {
	Version     int      `json:"version"`
	Connections []Record `json:"connections"`
}

func (s FileStore) Load() ([]Record, error) {
	f, err := os.Open(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return []Record{}, nil
	}
	if err != nil {
		return nil, errors.New("无法读取连接配置，请检查配置目录权限")
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || stat.Size() > 4<<20 {
		return nil, errors.New("连接配置文件过大或无法读取")
	}
	var doc document
	d := json.NewDecoder(io.LimitReader(f, (4<<20)+1))
	if err := d.Decode(&doc); err != nil {
		return nil, errors.New("连接配置格式损坏，原文件已保留，请先恢复配置文件")
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		return nil, errors.New("连接配置含多余内容，原文件已保留")
	}
	if doc.Version != 1 {
		return nil, fmt.Errorf("不支持版本 %d 的连接配置，原文件已保留", doc.Version)
	}
	seen := map[string]bool{}
	for _, r := range doc.Connections {
		if r.ID == "" || seen[r.ID] {
			return nil, errors.New("连接配置中存在无效或重复标识，原文件已保留")
		}
		seen[r.ID] = true
	}
	if doc.Connections == nil {
		return []Record{}, nil
	}
	return doc.Connections, nil
}

func (s FileStore) Save(records []Record) error {
	data, err := json.MarshalIndent(document{Version: 1, Connections: records}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return errors.New("无法创建配置目录")
	}
	f, err := os.CreateTemp(filepath.Dir(s.Path), ".connections-*.tmp")
	if err != nil {
		return errors.New("无法创建配置临时文件")
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return errors.New("连接配置写入失败，原配置已保留")
	}
	if err = os.Rename(tmp, s.Path); err != nil {
		return errors.New("无法替换连接配置，原配置已保留")
	}
	return nil
}
