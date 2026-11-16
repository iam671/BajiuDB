package connections

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type memoryRepo struct {
	records []Record
	groups  []Group
	fail    bool
}

func (r *memoryRepo) LoadGroups() ([]Group, error) { return append([]Group{}, r.groups...), nil }
func (r *memoryRepo) SaveGroups(groups []Group) error {
	if r.fail {
		return errors.New("disk full")
	}
	r.groups = append([]Group{}, groups...)
	return nil
}
func (r *memoryRepo) SaveWorkspace(records []Record, groups []Group) error {
	if r.fail {
		return errors.New("disk full")
	}
	r.records = append([]Record{}, records...)
	r.groups = append([]Group{}, groups...)
	return nil
}

func (r *memoryRepo) Load() ([]Record, error) { return append([]Record{}, r.records...), nil }
func (r *memoryRepo) Save(records []Record) error {
	if r.fail {
		return errors.New("disk full")
	}
	r.records = append([]Record{}, records...)
	return nil
}

type memorySecrets struct {
	values map[string]string
	fail   bool
}

func (s *memorySecrets) Get(key string) (string, error) {
	v, ok := s.values[key]
	if !ok {
		return "", errors.New("missing")
	}
	return v, nil
}
func (s *memorySecrets) Set(key, value string) error {
	if s.fail {
		return errors.New("locked")
	}
	s.values[key] = value
	return nil
}
func (s *memorySecrets) Delete(key string) error { delete(s.values, key); return nil }

type fakeDB struct {
	mu             sync.Mutex
	closed         bool
	fail           bool
	objectCalls    int
	objectsEntered chan struct{}
	objectsRelease chan struct{}
}

func (d *fakeDB) Objects(_ context.Context, database, kind string) (ObjectList, error) {
	d.objectCalls++
	if d.objectsEntered != nil {
		close(d.objectsEntered)
		<-d.objectsRelease
	}
	return ObjectList{Objects: []DatabaseObject{{Name: "customers", Kind: kind, Rows: "9007199254740993"}}}, nil
}

func TestObjectsRequiresAccessibleDatabaseAndLiveSession(t *testing.T) {
	s, _, _, dial := fixture()
	defer s.Close()
	p, err := s.Save(validInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Objects(p.ID, "test_db", "tables"); err == nil {
		t.Fatal("disconnected session accepted")
	}
	if _, err = s.Connect(Input{Profile: p, PasswordAction: "keep"}); err != nil {
		t.Fatal(err)
	}
	for _, request := range [][2]string{{"other_db", "tables"}, {"test_db' OR 1=1 --", "tables"}, {"test_db", "sql"}, {"", "tables"}} {
		if _, err = s.Objects(p.ID, request[0], request[1]); err == nil {
			t.Fatal("invalid metadata request accepted", request)
		}
	}
	if dial.db.objectCalls != 0 {
		t.Fatal("invalid request reached database")
	}
	result, err := s.Objects(p.ID, "test_db", "tables")
	if err != nil || len(result.Objects) != 1 || result.Objects[0].Rows != "9007199254740993" {
		t.Fatal("metadata lost", result, err)
	}
}

func TestObjectsRejectsResponseAfterDisconnect(t *testing.T) {
	s, _, _, dial := fixture()
	defer s.Close()
	p, err := s.Save(validInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Connect(Input{Profile: p, PasswordAction: "keep"}); err != nil {
		t.Fatal(err)
	}
	dial.db.objectsEntered, dial.db.objectsRelease = make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() { _, err := s.Objects(p.ID, "test_db", "tables"); done <- err }()
	<-dial.db.objectsEntered
	s.Disconnect(p.ID)
	close(dial.db.objectsRelease)
	if err = <-done; err == nil {
		t.Fatal("stale metadata accepted")
	}
}

func (d *fakeDB) Info(context.Context) (SessionInfo, error) {
	if d.fail {
		return SessionInfo{}, errors.New("server closed")
	}
	return SessionInfo{Connected: true, ServerVersion: "8.4-test", Databases: []string{"test_db"}}, nil
}
func (d *fakeDB) Close() error { d.mu.Lock(); defer d.mu.Unlock(); d.closed = true; return nil }

type fakeDialer struct {
	db       *fakeDB
	input    Input
	password string
	entered  chan struct{}
	release  chan struct{}
}

func (d *fakeDialer) Open(_ context.Context, in Input, password string) (Database, error) {
	d.input = in
	d.password = password
	if d.entered != nil {
		close(d.entered)
		<-d.release
	}
	return d.db, nil
}
func fixture() (*Service, *memoryRepo, *memorySecrets, *fakeDialer) {
	repo := &memoryRepo{}
	secrets := &memorySecrets{values: map[string]string{}}
	dial := &fakeDialer{db: &fakeDB{}}
	return New(repo, secrets, dial), repo, secrets, dial
}
func validInput() Input {
	return Input{Profile: Profile{Name: "本地测试", Host: "127.0.0.1", Port: 3306, Username: "test_user", Timeout: 5, TLSMode: "disabled", Environment: "local"}, Password: "test-password-only", RememberPassword: true, PasswordAction: "replace"}
}

func TestPasswordIsSeparatedAndPreservedOnEdit(t *testing.T) {
	s, repo, vault, dial := fixture()
	defer s.Close()
	in := validInput()
	p, err := s.Save(in)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(repo.records)
	if strings.Contains(string(data), in.Password) {
		t.Fatal("password leaked into config")
	}
	profiles, _ := s.List()
	serialized, _ := json.Marshal(profiles)
	if strings.Contains(string(serialized), "secretRef") || strings.Contains(string(serialized), in.Password) {
		t.Fatal("secret leaked into UI")
	}
	if !p.HasPassword || len(vault.values) != 1 {
		t.Fatal("password not saved")
	}
	edit := Input{Profile: p, RememberPassword: true, PasswordAction: "keep"}
	edit.Name = "已重命名"
	p, err = s.Save(edit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Test(Input{Profile: p, PasswordAction: "keep"}); err != nil {
		t.Fatal(err)
	}
	if dial.password != in.Password {
		t.Fatal("saved password not used")
	}
	if !dial.db.closed {
		t.Fatal("test connection leaked")
	}
}

func TestFailedConfigWriteKeepsOriginalSecret(t *testing.T) {
	s, repo, vault, _ := fixture()
	defer s.Close()
	p, err := s.Save(validInput())
	if err != nil {
		t.Fatal(err)
	}
	original := repo.records[0].SecretRef
	repo.fail = true
	_, err = s.Save(Input{Profile: p, RememberPassword: true, PasswordAction: "replace", Password: "replacement-test-only"})
	if err == nil {
		t.Fatal("expected write error")
	}
	if len(vault.values) != 1 || vault.values[original] != "test-password-only" {
		t.Fatal("failed write changed secret")
	}
}

func TestVaultFailureDoesNotSaveProfile(t *testing.T) {
	s, repo, vault, _ := fixture()
	defer s.Close()
	vault.fail = true
	if _, err := s.Save(validInput()); err == nil {
		t.Fatal("expected vault error")
	}
	if len(repo.records) != 0 {
		t.Fatal("partially saved profile")
	}
}

func TestRememberOffDoesNotStorePassword(t *testing.T) {
	s, repo, vault, _ := fixture()
	defer s.Close()
	in := validInput()
	in.RememberPassword = false
	p, err := s.Save(in)
	if err != nil {
		t.Fatal(err)
	}
	if p.HasPassword || len(vault.values) != 0 || repo.records[0].SecretRef != "" {
		t.Fatal("password persisted despite opt-out")
	}
}

func TestDeleteRemovesOnlyLocalProfileAndSecret(t *testing.T) {
	s, repo, vault, _ := fixture()
	defer s.Close()
	p, err := s.Save(validInput())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Delete(p.ID, p.Revision); err != nil {
		t.Fatal(err)
	}
	if len(repo.records) != 0 || len(vault.values) != 0 {
		t.Fatal("local record or secret not removed")
	}
}

func TestStaleRevisionAndDuplicateNamesAreRejected(t *testing.T) {
	s, _, _, _ := fixture()
	defer s.Close()
	p, _ := s.Save(validInput())
	in := validInput()
	in.Name = " 本地测试 "
	if _, err := s.Save(in); err == nil {
		t.Fatal("duplicate name accepted")
	}
	edit := Input{Profile: p, PasswordAction: "keep", RememberPassword: true}
	edit.Name = "新名字"
	if _, err := s.Save(edit); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(edit); err == nil {
		t.Fatal("stale revision accepted")
	}
}

func TestDisconnectCancelsInFlightConnect(t *testing.T) {
	s, _, _, dial := fixture()
	defer s.Close()
	p, _ := s.Save(validInput())
	dial.entered = make(chan struct{})
	dial.release = make(chan struct{})
	done := make(chan error, 1)
	go func() { _, err := s.Connect(Input{Profile: p, PasswordAction: "keep"}); done <- err }()
	<-dial.entered
	s.Disconnect(p.ID)
	close(dial.release)
	if err := <-done; err == nil {
		t.Fatal("canceled connect was accepted")
	}
	if len(s.Sessions()) != 0 || !dial.db.closed {
		t.Fatal("canceled session leaked")
	}
}

func TestRefreshFailureDropsDeadSession(t *testing.T) {
	s, _, _, dial := fixture()
	defer s.Close()
	p, _ := s.Save(validInput())
	if _, err := s.Connect(Input{Profile: p, PasswordAction: "keep"}); err != nil {
		t.Fatal(err)
	}
	dial.db.fail = true
	if _, err := s.Refresh(p.ID); err == nil {
		t.Fatal("expected refresh failure")
	}
	if len(s.Sessions()) != 0 || !dial.db.closed {
		t.Fatal("dead session still registered")
	}
}

func TestFileStoreRoundTripAndCorruptionProtection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "connections.json")
	store := FileStore{Path: path}
	s := New(store, &memorySecrets{values: map[string]string{}}, &fakeDialer{db: &fakeDB{}})
	in := validInput()
	in.RememberPassword = false
	if _, err := s.Save(in); err != nil {
		t.Fatal(err)
	}
	s.Close()
	second := New(store, &memorySecrets{values: map[string]string{}}, &fakeDialer{})
	records, err := second.List()
	if err != nil || len(records) != 1 {
		t.Fatal("restart lost connection", err)
	}
	second.Close()
	bad := []byte(`{"version":1,"connections":[`)
	if err = os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	broken := New(store, &memorySecrets{values: map[string]string{}}, &fakeDialer{})
	defer broken.Close()
	if _, err = broken.List(); err == nil {
		t.Fatal("corrupt file accepted")
	}
	if _, err = broken.Save(in); err == nil {
		t.Fatal("overwrote corrupt file")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(bad) {
		t.Fatal("original config changed")
	}
}

func TestInvalidEndpointsNeverReachDriver(t *testing.T) {
	for _, host := range []string{"mysql://localhost", "localhost:3306", "host/path", ""} {
		in := validInput()
		in.Host = host
		if _, err := normalize(in, false); err == nil {
			t.Fatalf("accepted invalid host %q", host)
		}
	}
	in := validInput()
	in.Host = "[::1]"
	if normalized, err := normalize(in, false); err != nil || normalized.Host != "::1" {
		t.Fatal("valid IPv6 rejected", err)
	}
}
