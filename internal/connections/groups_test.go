package connections

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGroupPersistencePreservesCredentialsAndSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connections.json")
	vault := &memorySecrets{values: map[string]string{}}
	dial := &fakeDialer{db: &fakeDB{}}
	s := New(FileStore{Path: path}, vault, dial)
	defer s.Close()
	// An empty group must persist independently of connections.
	group, err := s.SaveGroup("", "  工作  ")
	if err != nil || group.Name != "工作" {
		t.Fatal(group, err)
	}
	in := validInput()
	profile, err := s.Save(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Connect(Input{Profile: profile, PasswordAction: "keep"}); err != nil {
		t.Fatal(err)
	}
	moved, err := s.MoveToGroup(profile.ID, profile.Revision, group.ID)
	if err != nil || moved.Revision != profile.Revision+1 || !moved.HasPassword {
		t.Fatal(moved, err)
	}
	if len(s.Sessions()) != 1 || dial.db.closed || len(vault.values) != 1 {
		t.Fatal("moving changed session or credentials")
	}
	if _, err = s.MoveToGroup(profile.ID, profile.Revision, ""); err == nil {
		t.Fatal("stale move accepted")
	}
	if _, err = s.SaveGroup(group.ID, "项目组"); err != nil {
		t.Fatal(err)
	}
	restored := New(FileStore{Path: path}, vault, dial)
	defer restored.Close()
	groups, err := restored.Groups()
	if err != nil || len(groups) != 1 || groups[0].Name != "项目组" {
		t.Fatal(groups, err)
	}
	profiles, err := restored.List()
	if err != nil || len(profiles) != 1 || profiles[0].GroupID != group.ID || !profiles[0].HasPassword {
		t.Fatal(profiles, err)
	}
	bytes, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(bytes), in.Password) {
		t.Fatal("password leaked", err)
	}
	// Ordinary connection edits preserve the catalog in the same atomic file.
	edit := Input{Profile: profiles[0], PasswordAction: "keep", RememberPassword: true}
	edit.Name = "重新命名"
	if _, err = restored.Save(edit); err != nil {
		t.Fatal(err)
	}
	storedGroups, err := (FileStore{Path: path}).LoadGroups()
	if err != nil || len(storedGroups) != 1 {
		t.Fatal("connection save dropped groups", err)
	}
}

func TestGroupValidationAndFailedWritesKeepState(t *testing.T) {
	s, repo, _, _ := fixture()
	defer s.Close()
	group, err := s.SaveGroup("", "Work")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "work", "bad\nname", strings.Repeat("字", 81)} {
		if _, err = s.SaveGroup("", name); err == nil {
			t.Fatal("invalid group accepted", name)
		}
	}
	in := validInput()
	in.GroupID = "missing"
	if _, err = s.Save(in); err == nil {
		t.Fatal("unknown group accepted")
	}
	in.GroupID = group.ID
	profile, err := s.Save(in)
	if err != nil {
		t.Fatal(err)
	}
	repo.fail = true
	if _, err = s.SaveGroup(group.ID, "changed"); err == nil {
		t.Fatal("rename write failure ignored")
	}
	if _, err = s.MoveToGroup(profile.ID, profile.Revision, ""); err == nil {
		t.Fatal("move write failure ignored")
	}
	groups, _ := s.Groups()
	profiles, _ := s.List()
	if groups[0].Name != "Work" || profiles[0].GroupID != group.ID || profiles[0].Revision != profile.Revision {
		t.Fatal("failed write changed memory")
	}
}

func TestDisconnectAllClosesSessionsAndCancelsPendingConnect(t *testing.T) {
	s, _, _, dial := fixture()
	defer s.Close()
	p, err := s.Save(validInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Connect(Input{Profile: p, PasswordAction: "keep"}); err != nil {
		t.Fatal(err)
	}
	firstDB := dial.db
	dial.db = &fakeDB{}
	secondInput := validInput()
	secondInput.Name = "另一连接"
	second, err := s.Save(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Connect(Input{Profile: second, PasswordAction: "keep"}); err != nil {
		t.Fatal(err)
	}
	s.DisconnectAll()
	if len(s.Sessions()) != 0 || !dial.db.closed || !firstDB.closed {
		t.Fatal("live session was not closed")
	}
	dial.db = &fakeDB{}
	dial.entered = make(chan struct{})
	dial.release = make(chan struct{})
	done := make(chan error, 1)
	go func() { _, err := s.Connect(Input{Profile: p, PasswordAction: "keep"}); done <- err }()
	<-dial.entered
	s.DisconnectAll()
	close(dial.release)
	if err = <-done; err == nil || len(s.Sessions()) != 0 || !dial.db.closed {
		t.Fatal("pending connect survived close all", err)
	}
	if profiles, _ := s.List(); len(profiles) != 2 {
		t.Fatal("close all removed saved profile")
	}
}

func TestDeleteGroupPersistsConnectionsAtRootWithoutClosingSession(t *testing.T) {
	store := FileStore{Path: filepath.Join(t.TempDir(), "connections.json")}
	vault := &memorySecrets{values: map[string]string{}}
	dial := &fakeDialer{db: &fakeDB{}}
	s := New(store, vault, dial)
	defer s.Close()
	group, err := s.SaveGroup("", "删除此组")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.SaveGroup("", "保留此组")
	if err != nil {
		t.Fatal(err)
	}
	in := validInput()
	in.GroupID = group.ID
	profile, err := s.Save(in)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Connect(Input{Profile: profile, PasswordAction: "keep"}); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteGroup(group.ID); err != nil {
		t.Fatal(err)
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	groups, err := store.LoadGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].ID != other.ID {
		t.Fatal("deleted wrong group", groups)
	}
	if len(after) != 1 || after[0].GroupID != "" || after[0].ID != profile.ID || after[0].Revision != profile.Revision+1 || after[0].SecretRef != before[0].SecretRef {
		t.Fatal("connection was not preserved at root", after)
	}
	if len(vault.values) != 1 || len(s.Sessions()) != 1 || dial.db.closed {
		t.Fatal("deleting group changed credential or session")
	}
	restored := New(store, vault, dial)
	defer restored.Close()
	profiles, err := restored.List()
	if err != nil || len(profiles) != 1 || profiles[0].GroupID != "" || !profiles[0].HasPassword {
		t.Fatal("invalid state after reload", profiles, err)
	}
	if err = s.DeleteGroup(other.ID); err != nil {
		t.Fatal("empty group delete", err)
	}
	if err = s.DeleteGroup(""); err == nil {
		t.Fatal("root deletion allowed")
	}
}

func TestDeleteGroupFailureLeavesGroupAndConnectionUnchanged(t *testing.T) {
	s, repo, vault, _ := fixture()
	defer s.Close()
	group, err := s.SaveGroup("", "原组")
	if err != nil {
		t.Fatal(err)
	}
	in := validInput()
	in.GroupID = group.ID
	profile, err := s.Save(in)
	if err != nil {
		t.Fatal(err)
	}
	repo.fail = true
	if err = s.DeleteGroup(group.ID); err == nil {
		t.Fatal("failed delete ignored")
	}
	groups, _ := s.Groups()
	profiles, _ := s.List()
	if len(groups) != 1 || groups[0].ID != group.ID || profiles[0].GroupID != group.ID || profiles[0].Revision != profile.Revision || len(vault.values) != 1 {
		t.Fatal("failed write partially deleted the group")
	}
	if len(repo.groups) != 1 || repo.records[0].GroupID != group.ID {
		t.Fatal("repository partially changed")
	}
}
