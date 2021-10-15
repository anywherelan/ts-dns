package main

import (
	"bytes"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed files_replacement/interfaces.go
var interfacesFile []byte

//go:embed files_replacement/manager.go
var managerFile []byte

const (
	upstreamRepo = "git@github.com:tailscale/tailscale.git"
	cloneDir     = "temp-clone-dir"
	outputDir    = "temp-output-dir"
)

func main() {
	runCommand("git", "clone", upstreamRepo, cloneDir)
	os.Mkdir(outputDir, 0777)
	defer runCommand("rm", "-rf", cloneDir)
	defer runCommand("rm", "-rf", outputDir)

	copyFiles(filepath.Join(cloneDir, "AUTHORS"), outputDir)
	copyFiles(filepath.Join(cloneDir, "LICENSE"), outputDir)
	copyFiles(filepath.Join(cloneDir, "go.mod"), outputDir)
	copyFiles(filepath.Join(cloneDir, "go.sum"), outputDir)

	netOutputPath := filepath.Join(outputDir, "net")
	os.MkdirAll(netOutputPath, 0777)
	copyFiles(filepath.Join(cloneDir, "net", "dns"), netOutputPath)
	removeFiles(filepath.Join(outputDir, "net", "dns"), "resolver", "manager.go", "manager_test.go", "manager_linux_test.go")
	os.WriteFile(filepath.Join(outputDir, "net", "dns", "manager.go"), managerFile, 0666)

	netInterfacesOutputPath := filepath.Join(netOutputPath, "interfaces")
	os.MkdirAll(netInterfacesOutputPath, 0777)
	os.WriteFile(filepath.Join(netInterfacesOutputPath, "interfaces.go"), interfacesFile, 0666)

	utilOutputPath := filepath.Join(outputDir, "util")
	os.MkdirAll(utilOutputPath, 0777)
	copyFiles(filepath.Join(cloneDir, "util", "dnsname"), utilOutputPath)
	copyFiles(filepath.Join(cloneDir, "util", "cmpver"), utilOutputPath)
	copyFiles(filepath.Join(cloneDir, "util", "endian"), utilOutputPath)
	copyFiles(filepath.Join(cloneDir, "util", "winutil"), utilOutputPath)

	copyFiles(filepath.Join(cloneDir, "atomicfile"), outputDir)

	typesOutputPath := filepath.Join(outputDir, "types")
	os.MkdirAll(typesOutputPath, 0777)
	copyFiles(filepath.Join(cloneDir, "types", "logger"), typesOutputPath)
	copyFiles(filepath.Join(cloneDir, "types", "dnstype"), typesOutputPath)

	recursiveReplaceInFiles(outputDir, `"tailscale.com/`, `"github.com/anywherelan/ts-dns/`)
	recursiveReplaceInFiles(filepath.Join(outputDir, "go.mod"), "module tailscale.com", "module github.com/anywherelan/ts-dns")

	os.Chdir(outputDir)
	runCommand("go", "mod", "tidy")
	runCommand("go", "build", "./...")
	os.Chdir("..")

	dirEntries, err := os.ReadDir(outputDir)
	if err != nil {
		panic(err)
	}
	for _, entry := range dirEntries {
		copyFiles(filepath.Join(outputDir, entry.Name()), ".")
	}
}

func copyFiles(src, dst string) {
	runCommand("cp", "-r", src, dst)
}

func removeFiles(basePath string, files ...string) {
	for _, file := range files {
		os.RemoveAll(filepath.Join(basePath, file))
	}
}

func recursiveReplaceInFiles(root, from, to string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		data, _ := os.ReadFile(path)
		data = bytes.ReplaceAll(data, []byte(from), []byte(to))
		os.WriteFile(path, data, info.Mode())

		return nil
	})
}

func runCommand(command string, params ...string) error {
	cmd := exec.Command(command, params...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}
