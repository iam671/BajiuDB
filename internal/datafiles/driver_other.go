//go:build !windows

package datafiles

import (
	"context"
	"errors"
)

func driverCapabilities(context.Context) (bool, bool, string) {
	return false, false, "XLS、Access、ODBC、DBF 适配器当前需要 Windows 系统驱动；其它六种格式可跨平台使用"
}
func driverInspect(context.Context, Options) ([]string, error) {
	return nil, errors.New("该格式需要 Windows 数据驱动")
}
func driverRead(context.Context, Options) (Data, error) {
	return Data{}, errors.New("该格式需要 Windows 数据驱动")
}
func driverWrite(context.Context, Options, Data) error {
	return errors.New("该格式需要 Windows 数据驱动")
}
