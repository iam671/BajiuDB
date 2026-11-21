package connections

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Repository interface {
	Load() ([]Record, error)
	Save([]Record) error
}

type FileStore struct{ Path string }
type document struct {
	Version     int      `json:"version"`
	Connections []Record `json:"connections"`
	Groups      []Group  `json:"groups,omitempty"`
}

func (s FileStore) Load() ([]Record, error) {
	doc, err := s.readDocument()
	return doc.Connections, err
}

func (s FileStore) readDocument() (document, error) {
	empty := document{Version: 1, Connections: []Record{}}
	f, err := os.Open(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, errors.New("无法读取连接配置，请检查配置目录权限")
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || stat.Size() > 4<<20 {
		return empty, errors.New("连接配置文件过大或无法读取")
	}
	var doc document
	d := json.NewDecoder(io.LimitReader(f, (4<<20)+1))
	if err := d.Decode(&doc); err != nil {
		return empty, errors.New("连接配置格式损坏，原文件已保留，请先恢复配置文件")
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		return empty, errors.New("连接配置含多余内容，原文件已保留")
	}
	if doc.Version != 1 {
		return empty, fmt.Errorf("不支持版本 %d 的连接配置，原文件已保留", doc.Version)
	}
	groupIDs, groupNames := map[string]bool{}, map[string]bool{}
	for _, group := range doc.Groups {
		name, nameErr := groupName(group.Name)
		if group.ID == "" || groupIDs[group.ID] || nameErr != nil || groupNames[strings.ToLower(name)] {
			return empty, errors.New("连接分组配置无效，原文件已保留")
		}
		groupIDs[group.ID], groupNames[strings.ToLower(name)] = true, true
	}
	seen := map[string]bool{}
	for i := range doc.Connections {
		r := doc.Connections[i]
		r.Engine = canonicalEngine(r.Engine)
		if _, ok := engineCatalog[r.Engine]; !ok {
			return empty, errors.New("连接类型无效，原文件已保留")
		}
		doc.Connections[i] = r
		if r.ID == "" || seen[r.ID] {
			return empty, errors.New("连接配置中存在无效或重复标识，原文件已保留")
		}
		if r.GroupID != "" && !groupIDs[r.GroupID] {
			return empty, errors.New("连接引用了不存在的分组，原文件已保留")
		}
		seen[r.ID] = true
	}
	if doc.Connections == nil {
		doc.Connections = []Record{}
	}
	return doc, nil
}

func (s FileStore) Save(records []Record) error {
	doc, err := s.readDocument()
	if err != nil {
		return err
	}
	doc.Connections = records
	return s.writeDocument(doc)
}

func (s FileStore) LoadGroups() ([]Group, error) {
	doc, err := s.readDocument()
	if doc.Groups == nil {
		doc.Groups = []Group{}
	}
	return doc.Groups, err
}

func (s FileStore) SaveGroups(groups []Group) error {
	doc, err := s.readDocument()
	if err != nil {
		return err
	}
	doc.Groups = groups
	return s.writeDocument(doc)
}

func (s FileStore) SaveWorkspace(records []Record, groups []Group) error {
	doc, err := s.readDocument()
	if err != nil {
		return err
	}
	doc.Connections, doc.Groups = records, groups
	return s.writeDocument(doc)
}

func (s FileStore) writeDocument(doc document) error {
	data, err := json.MarshalIndent(doc, "", "  ")
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
