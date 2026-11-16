package connections

import (
	"encoding/json"
	"errors"
)

type secretValues struct {
	MySQL            string `json:"mysql"`
	SSHPassword      string `json:"sshPassword"`
	SSHKeyPassphrase string `json:"sshKeyPassphrase"`
	HTTPToken        string `json:"httpToken"`
}

func (s *Service) readSecrets(record Record) (secretValues, error) {
	if record.SecretRef == "" {
		return secretValues{}, nil
	}
	raw, err := s.secrets.Get(record.SecretRef)
	if err != nil {
		return secretValues{}, errors.New("无法从系统凭据库读取密码，请重新输入密码")
	}
	if record.SecretFormat == 0 {
		return secretValues{MySQL: raw}, nil
	}
	var values secretValues
	if record.SecretFormat != 1 || json.Unmarshal([]byte(raw), &values) != nil {
		return values, errors.New("已存凭据格式无效，请重新输入")
	}
	return values, nil
}
func mergeSecrets(in Input, old secretValues) secretValues {
	if in.PasswordAction == "replace" {
		old.MySQL = in.Password
	} else if in.PasswordAction == "forget" {
		old.MySQL = ""
	}
	if in.SSHPasswordAction == "replace" {
		old.SSHPassword = in.SSHPassword
	}
	if in.SSHKeyPassphraseAction == "replace" {
		old.SSHKeyPassphrase = in.SSHKeyPassphrase
	}
	if in.HTTPTokenAction == "replace" {
		old.HTTPToken = in.HTTPToken
	}
	return old
}
func allSecretsKept(in Input) bool {
	return in.PasswordAction == "keep" && in.SSHPasswordAction == "keep" && in.SSHKeyPassphraseAction == "keep" && in.HTTPTokenAction == "keep"
}
func needsOldSecrets(in Input) bool {
	return in.PasswordAction == "keep" || in.SSHPasswordAction == "keep" || in.SSHKeyPassphraseAction == "keep" || in.HTTPTokenAction == "keep"
}
func (s *Service) resolveCredentials(in Input) (Input, string, error) {
	var old secretValues
	if in.ID != "" && needsOldSecrets(in) {
		s.mu.Lock()
		index := s.index(in.ID)
		if index < 0 {
			s.mu.Unlock()
			return in, "", errors.New("连接已被删除，请刷新列表")
		}
		record := s.records[index]
		s.mu.Unlock()
		if record.Revision != in.Revision {
			return in, "", errors.New("连接配置已变化，请重新选择")
		}
		var err error
		old, err = s.readSecrets(record)
		if err != nil {
			return in, "", err
		}
	}
	values := mergeSecrets(in, old)
	in.SSHPassword, in.SSHKeyPassphrase, in.HTTPToken = values.SSHPassword, values.SSHKeyPassphrase, values.HTTPToken
	return in, values.MySQL, nil
}
