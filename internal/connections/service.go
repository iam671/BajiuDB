package connections

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

type liveSession struct {
	db      Database
	info    SessionInfo
	timeout int
}

type Service struct {
	mu       sync.Mutex
	repo     Repository
	secrets  Secrets
	dialer   Dialer
	records  []Record
	groups   []Group
	sessions map[string]liveSession
	epoch    map[string]uint64
	ctx      context.Context
	cancel   context.CancelFunc
	loadErr  error
}

func New(repo Repository, secrets Secrets, dialer Dialer) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	r, err := repo.Load()
	groups := []Group{}
	if catalog, ok := repo.(groupRepository); ok && err == nil {
		groups, err = catalog.LoadGroups()
	}
	return &Service{repo: repo, secrets: secrets, dialer: dialer, records: r, groups: groups, sessions: map[string]liveSession{}, epoch: map[string]uint64{}, ctx: ctx, cancel: cancel, loadErr: err}
}

func newID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", errors.New("无法生成连接标识，请重试")
	}
	return hex.EncodeToString(bytes[:]), nil
}

func (s *Service) index(id string) int {
	for i, r := range s.records {
		if r.ID == id {
			return i
		}
	}
	return -1
}

func (s *Service) List() ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	out := make([]Profile, 0, len(s.records))
	for _, r := range s.records {
		p := r.Profile
		p.HasPassword = r.SecretRef != ""
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Service) Save(raw Input) (Profile, error) {
	in, err := normalize(raw, true)
	if err != nil {
		return Profile{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return Profile{}, s.loadErr
	}
	old := Record{}
	if !s.hasGroup(in.GroupID) {
		return Profile{}, errors.New("连接分组不存在，请重新选择分组")
	}
	index := -1
	if in.ID != "" {
		index = s.index(in.ID)
		if index < 0 {
			return Profile{}, errors.New("此连接已被删除，请刷新列表")
		}
		old = s.records[index]
		if old.Revision != in.Revision {
			return Profile{}, errors.New("连接配置已变化，请重新选择连接后编辑")
		}
	}
	for _, r := range s.records {
		if r.ID != in.ID && strings.EqualFold(r.Name, in.Name) {
			return Profile{}, errors.New("已有同名连接，请换一个名称")
		}
	}
	p := in.Profile
	p.CreatedAt = old.CreatedAt
	p.UpdatedAt = time.Now().UTC()
	p.Revision = old.Revision + 1
	if index < 0 {
		p.ID, err = newID()
		if err != nil {
			return Profile{}, err
		}
		p.CreatedAt = p.UpdatedAt
	}
	ref := old.SecretRef
	createdRef := ""
	format := old.SecretFormat
	if !in.RememberPassword || in.PasswordAction == "forget" {
		ref = ""
		format = 0
	} else if !allSecretsKept(in) {
		values := secretValues{}
		if needsOldSecrets(in) && old.SecretRef != "" {
			values, err = s.readSecrets(old)
			if err != nil {
				return Profile{}, err
			}
		}
		values = mergeSecrets(in, values)
		raw := values.MySQL
		format = 0
		if values.SSHPassword != "" || values.SSHKeyPassphrase != "" || values.HTTPToken != "" {
			encoded, _ := json.Marshal(values)
			raw = string(encoded)
			format = 1
		}
		ref = ""
		if raw != "" {
			createdRef, err = newID()
			if err != nil {
				return Profile{}, err
			}
			if err = s.secrets.Set(createdRef, raw); err != nil {
				return Profile{}, errors.New("系统凭据库无法保存密码；可关闭“记住密码”后保存连接")
			}
			ref = createdRef
		}
	}
	p.HasPassword = ref != ""
	updated := Record{Profile: p, SecretRef: ref, SecretFormat: format}
	next := append([]Record{}, s.records...)
	if index < 0 {
		next = append(next, updated)
	} else {
		next[index] = updated
	}
	if err = s.repo.Save(next); err != nil {
		if createdRef != "" {
			_ = s.secrets.Delete(createdRef)
		}
		return Profile{}, err
	}
	s.records = next
	s.epoch[p.ID]++
	if live, ok := s.sessions[p.ID]; ok {
		_ = live.db.Close()
		delete(s.sessions, p.ID)
	}
	if old.SecretRef != "" && old.SecretRef != ref {
		_ = s.secrets.Delete(old.SecretRef)
	}
	return p, nil
}

func (s *Service) Delete(id string, revision int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return s.loadErr
	}
	i := s.index(id)
	if i < 0 {
		return errors.New("此连接已被删除")
	}
	old := s.records[i]
	if old.Revision != revision {
		return errors.New("连接配置已变化，请刷新后再删除")
	}
	next := make([]Record, 0, len(s.records)-1)
	next = append(next, s.records[:i]...)
	next = append(next, s.records[i+1:]...)
	if err := s.repo.Save(next); err != nil {
		return err
	}
	s.records = next
	s.epoch[id]++
	if live, ok := s.sessions[id]; ok {
		_ = live.db.Close()
		delete(s.sessions, id)
	}
	if old.SecretRef != "" {
		_ = s.secrets.Delete(old.SecretRef)
	}
	return nil
}

func (s *Service) open(raw Input) (Database, SessionInfo, error) {
	in, err := normalize(raw, false)
	if err != nil {
		return nil, SessionInfo{}, err
	}
	in, password, err := s.resolveCredentials(in)
	if err != nil {
		return nil, SessionInfo{}, err
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(in.Timeout)*time.Second)
	defer cancel()
	start := time.Now()
	db, err := s.dialer.Open(ctx, in, password)
	if err != nil {
		return nil, SessionInfo{}, err
	}
	info, err := db.Info(ctx)
	if err != nil {
		_ = db.Close()
		return nil, SessionInfo{}, err
	}
	info.ID = in.ID
	info.LatencyMS = time.Since(start).Milliseconds()
	info.ConnectedAt = time.Now().UTC()
	return db, info, nil
}

func (s *Service) Test(in Input) (TestResult, error) {
	db, info, err := s.open(in)
	if err != nil {
		return TestResult{}, err
	}
	defer db.Close()
	return TestResult{Success: true, Message: "连接成功，已验证登录和数据库访问", ServerVersion: info.ServerVersion, LatencyMS: info.LatencyMS}, nil
}

func (s *Service) Connect(in Input) (SessionInfo, error) {
	s.mu.Lock()
	i := s.index(in.ID)
	if i < 0 || s.records[i].Revision != in.Revision {
		s.mu.Unlock()
		return SessionInfo{}, errors.New("请先保存当前连接配置")
	}
	s.epoch[in.ID]++
	epoch := s.epoch[in.ID]
	s.mu.Unlock()
	db, info, err := s.open(in)
	if err != nil {
		return SessionInfo{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.epoch[in.ID] != epoch || s.ctx.Err() != nil {
		_ = db.Close()
		return SessionInfo{}, errors.New("连接操作已取消或配置已变化")
	}
	if old, ok := s.sessions[in.ID]; ok {
		_ = old.db.Close()
	}
	s.sessions[in.ID] = liveSession{db: db, info: info, timeout: in.Timeout}
	return info, nil
}

func (s *Service) Disconnect(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epoch[id]++
	if old, ok := s.sessions[id]; ok {
		_ = old.db.Close()
		delete(s.sessions, id)
	}
}

func (s *Service) Sessions() []SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []SessionInfo{}
	for _, live := range s.sessions {
		result = append(result, live.info)
	}
	return result
}

func (s *Service) Refresh(id string) (SessionInfo, error) {
	s.mu.Lock()
	live, ok := s.sessions[id]
	epoch := s.epoch[id]
	s.mu.Unlock()
	if !ok {
		return SessionInfo{}, errors.New("请先连接数据库")
	}
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(live.timeout)*time.Second)
	defer cancel()
	info, err := live.db.Info(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.epoch[id] != epoch {
		return SessionInfo{}, errors.New("此连接已断开或配置已变化")
	}
	if err != nil {
		_ = live.db.Close()
		delete(s.sessions, id)
		return SessionInfo{}, err
	}
	info.ID, info.ConnectedAt, info.LatencyMS = id, live.info.ConnectedAt, live.info.LatencyMS
	live.info = info
	s.sessions[id] = live
	return info, nil
}

func (s *Service) Close() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, live := range s.sessions {
		_ = live.db.Close()
		delete(s.sessions, id)
	}
}
