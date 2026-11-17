package main

import (
	"encoding/json"
	"fmt"
	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
	"os"
	"strings"
)

func run() error {
	config, err := os.ReadFile("wails.json")
	if err != nil {
		return err
	}
	var metadata struct {
		Info struct {
			CompanyName    string `json:"companyName"`
			ProductName    string `json:"productName"`
			ProductVersion string `json:"productVersion"`
		}
	}
	if err = json.Unmarshal(config, &metadata); err != nil {
		return err
	}
	info, err := os.ReadFile("build/windows/info.json")
	if err != nil {
		return err
	}
	replacer := strings.NewReplacer("{{.Info.ProductVersion}}", metadata.Info.ProductVersion, "{{.Info.CompanyName}}", metadata.Info.CompanyName, "{{.Info.ProductName}}", metadata.Info.ProductName, "{{.Info.Copyright}}", "", "{{.Info.Comments}}", "Bajiu DB MySQL desktop client")
	var v version.Info
	if err = v.UnmarshalJSON([]byte(replacer.Replace(string(info)))); err != nil {
		return err
	}
	iconFile, err := os.Open("build/windows/icon.ico")
	if err != nil {
		return err
	}
	defer iconFile.Close()
	icon, err := winres.LoadICO(iconFile)
	if err != nil {
		return err
	}
	manifestData, err := os.ReadFile("build/windows/wails.exe.manifest")
	if err != nil {
		return err
	}
	manifest, err := winres.AppManifestFromXML(manifestData)
	if err != nil {
		return err
	}
	resources := winres.ResourceSet{}
	if err = resources.SetIcon(winres.RT_ICON, icon); err != nil {
		return err
	}
	resources.SetManifest(manifest)
	resources.SetVersionInfo(v)
	output, err := os.Create("rsrc_windows_amd64.syso")
	if err != nil {
		return err
	}
	defer output.Close()
	return resources.WriteObject(output, winres.ArchAMD64)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
