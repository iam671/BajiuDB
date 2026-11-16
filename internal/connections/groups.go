package connections

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type groupRepository interface {
	LoadGroups() ([]Group, error)
	SaveGroups([]Group) error
}
type groupWorkspaceRepository interface {
	SaveWorkspace([]Record, []Group) error
}

func groupName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", errors.New("组名需要填写，最多 80 个字，不能包含换行或控制字符")
	}
	return name, nil
}
func (s *Service) hasGroup(id string) bool {
	if id == "" {
		return true
	}
	for _, group := range s.groups {
		if group.ID == id {
			return true
		}
	}
	return false
}
func (s *Service) Groups() ([]Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return append([]Group{}, s.groups...), nil
}

// SaveGroup creates a group when id is empty, otherwise renames the existing group.
func (s *Service) SaveGroup(id, rawName string) (Group, error) {
	name, err := groupName(rawName)
	if err != nil {
		return Group{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return Group{}, s.loadErr
	}
	repo, ok := s.repo.(groupRepository)
	if !ok {
		return Group{}, errors.New("当前配置存储不支持连接分组")
	}
	index := -1
	for i, group := range s.groups {
		if group.ID == id {
			index = i
		}
		if group.ID != id && strings.EqualFold(group.Name, name) {
			return Group{}, errors.New("已有同名分组，请换一个名称")
		}
	}
	if id != "" && index < 0 {
		return Group{}, errors.New("此分组已不存在，请刷新列表")
	}
	if id == "" {
		if len(s.groups) >= 1000 {
			return Group{}, errors.New("连接分组已达到数量上限")
		}
		id, err = newID()
		if err != nil {
			return Group{}, err
		}
	}
	group := Group{ID: id, Name: name}
	next := append([]Group{}, s.groups...)
	if index < 0 {
		next = append(next, group)
	} else {
		next[index] = group
	}
	if err = repo.SaveGroups(next); err != nil {
		return Group{}, err
	}
	s.groups = next
	return group, nil
}

// Moving a connection changes only its local metadata, preserving credentials and live sessions.
func (s *Service) MoveToGroup(id string, revision int, groupID string) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return Profile{}, s.loadErr
	}
	index := s.index(id)
	if index < 0 {
		return Profile{}, errors.New("此连接已被删除，请刷新列表")
	}
	old := s.records[index]
	if old.Revision != revision {
		return Profile{}, errors.New("连接配置已变化，请重新选择连接")
	}
	if !s.hasGroup(groupID) {
		return Profile{}, errors.New("连接分组不存在，请重新选择分组")
	}
	if old.GroupID == groupID {
		return old.Profile, nil
	}
	next := append([]Record{}, s.records...)
	next[index].GroupID = groupID
	next[index].Revision++
	next[index].UpdatedAt = time.Now().UTC()
	if err := s.repo.Save(next); err != nil {
		return Profile{}, err
	}
	s.records = next
	profile := next[index].Profile
	profile.HasPassword = next[index].SecretRef != ""
	return profile, nil
}

// DeleteGroup atomically removes the folder and moves its connections to the root.
// Credentials and open sessions are unaffected.
func (s *Service) DeleteGroup(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return s.loadErr
	}
	if id == "" || !s.hasGroup(id) {
		return errors.New("此分组已不存在，请刷新列表")
	}
	repo, ok := s.repo.(groupWorkspaceRepository)
	if !ok {
		return errors.New("当前配置存储不支持删除分组")
	}
	groups := make([]Group, 0, len(s.groups)-1)
	for _, group := range s.groups {
		if group.ID != id {
			groups = append(groups, group)
		}
	}
	records := append([]Record{}, s.records...)
	now := time.Now().UTC()
	for i := range records {
		if records[i].GroupID == id {
			records[i].GroupID = ""
			records[i].Revision++
			records[i].UpdatedAt = now
		}
	}
	if err := repo.SaveWorkspace(records, groups); err != nil {
		return err
	}
	s.records, s.groups = records, groups
	return nil
}

func (s *Service) DisconnectAll() {
	s.mu.Lock()
	for id := range s.epoch {
		s.epoch[id]++
	}
	databases := make([]Database, 0, len(s.sessions))
	for _, live := range s.sessions {
		databases = append(databases, live.db)
	}
	s.sessions = map[string]liveSession{}
	s.mu.Unlock()
	for _, db := range databases {
		_ = db.Close()
	}
}
