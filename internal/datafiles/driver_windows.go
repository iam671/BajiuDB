//go:build windows

package datafiles

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

//go:embed transfer_windows.ps1
var driverScript string

//go:embed templates/blank.mdb
var emptyMDB []byte

//go:embed templates/blank.accdb
var emptyACCDB []byte

type driverResponse struct {
	Tables []string `json:"tables"`
	Data   Data     `json:"data"`
	ACE    bool     `json:"ace"`
	ODBC   bool     `json:"odbc"`
	Error  string   `json:"error"`
}
type cappedOutput struct {
	bytes.Buffer
	limit int
}

func (b *cappedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, errors.New("驱动返回内容超限")
	}
	return b.Buffer.Write(p)
}

type driverStreamRequest struct {
	Path    string `json:"path"`
	Lengths []int  `json:"lengths"`
	Count   int64  `json:"count"`
	Digest  string `json:"digest"`
}

func driverCall(ctx context.Context, operation string, o Options, d Data, stream ...*driverStreamRequest) (driverResponse, error) {
	deadline := 2 * time.Minute
	var info *driverStreamRequest
	if len(stream) > 0 {
		info = stream[0]
		deadline = 24 * time.Hour
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	input, err := json.Marshal(struct {
		Operation string               `json:"operation"`
		Options   Options              `json:"options"`
		Data      Data                 `json:"data"`
		Stream    *driverStreamRequest `json:"stream,omitempty"`
	}{operation, o, d, info})
	if err != nil {
		return driverResponse{}, err
	}
	executable := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.CommandContext(ctx, executable, "-NoProfile", "-NonInteractive", "-Command", driverScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.Stdin = bytes.NewReader(input)
	out := &cappedOutput{limit: 64 << 20}
	stderr := &cappedOutput{limit: 4096}
	cmd.Stdout = out
	cmd.Stderr = stderr
	if err = cmd.Run(); err != nil {
		var reported driverResponse
		if json.Unmarshal(out.Bytes(), &reported) == nil && reported.Error != "" {
			return reported, errors.New(reported.Error)
		}
		if ctx.Err() != nil {
			return driverResponse{}, errors.New("数据驱动操作已停止或超时，请检查驱动和数据源")
		}
		return driverResponse{}, fmt.Errorf("无法运行系统数据驱动，请检查 Windows PowerShell/.NET：%w", err)
	}
	var response driverResponse
	if err = json.Unmarshal(bytes.TrimPrefix(out.Bytes(), []byte{239, 187, 191}), &response); err != nil {
		return response, errors.New("系统数据驱动未返回有效结果")
	}
	if response.Error != "" {
		return response, errors.New(response.Error)
	}
	return response, nil
}
func driverCapabilities(ctx context.Context) (bool, bool, string) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	r, err := driverCall(ctx, "capabilities", Options{}, Data{})
	if err != nil {
		return false, false, "系统数据驱动不可用：" + err.Error()
	}
	return r.ACE, r.ODBC, "需要与本程序同为 64 位的 Microsoft Access Database Engine (ACE)；不随应用分发"
}
func driverInspect(ctx context.Context, o Options) ([]string, error) {
	r, e := driverCall(ctx, "inspect", o, Data{})
	return r.Tables, e
}
func driverRead(ctx context.Context, o Options) (Data, error) {
	r, e := driverCall(ctx, "read", o, Data{})
	return r.Data, e
}
func driverWrite(ctx context.Context, o Options, d Data) error {
	if err := prepareDriverFile(o); err != nil {
		return err
	}
	_, e := driverCall(ctx, "write", o, d)
	return e
}
func prepareDriverFile(o Options) error {
	if o.Format == "mdb" || o.Format == "accdb" {
		blank := emptyMDB
		if o.Format == "accdb" {
			blank = emptyACCDB
		}
		file, err := os.OpenFile(o.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, err = file.Write(blank)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
func driverWriteStream(ctx context.Context, o Options, source Source) error {
	spool, err := os.CreateTemp("", "bajiu-export-*.jsonl")
	if err != nil {
		return err
	}
	defer os.Remove(spool.Name())
	defer spool.Close()
	encoder := json.NewEncoder(spool)
	info := &driverStreamRequest{Path: spool.Name()}
	var names []string
	var digest [32]byte
	err = source(func(columns []string) error {
		names = append([]string{}, columns...)
		info.Lengths = make([]int, len(names))
		return nil
	}, func(row []*string) error {
		for i, v := range row {
			if v != nil {
				n := 0
				for _, r := range *v {
					n++
					if r > 0xffff {
						n++
					}
				}
				if n > info.Lengths[i] {
					info.Lengths[i] = n
				}
			}
		}
		addRowDigest(&digest, row)
		info.Count++
		return encoder.Encode(struct {
			Values []*string `json:"values"`
		}{row})
	})
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return errors.New("导出没有返回字段列表")
	}
	if err = spool.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	info.Digest = base64.StdEncoding.EncodeToString(digest[:])
	if err = prepareDriverFile(o); err != nil {
		return err
	}
	_, err = driverCall(ctx, "stream-write", o, Data{Columns: names}, info)
	return err
}
