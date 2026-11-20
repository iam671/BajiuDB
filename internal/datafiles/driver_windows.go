//go:build windows

package datafiles

import (
	"bytes"
	"context"
	_ "embed"
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
func driverCall(ctx context.Context, operation string, o Options, d Data) (driverResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	input, err := json.Marshal(struct {
		Operation string  `json:"operation"`
		Options   Options `json:"options"`
		Data      Data    `json:"data"`
	}{operation, o, d})
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
			return driverResponse{}, errors.New("数据驱动操作已停止或超过两分钟，请检查驱动和数据源")
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
	_, e := driverCall(ctx, "write", o, d)
	return e
}
