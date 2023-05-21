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

//go:embed files_replacement/envknob.go
var envknobFile []byte

//go:embed files_replacement/health.go
var healthFile []byte

const (
	upstreamRepo = "git@github.com:tailscale/tailscale.git"
	cloneDir     = "temp-clone-dir"
	outputDir    = "temp-output-dir"
)

func main() {
	runCommand("git", "clone", "--depth", "1", upstreamRepo, cloneDir)
	os.Mkdir(outputDir, 0777)
	defer runCommand("rm", "-rf", cloneDir)
	defer runCommand("rm", "-rf", outputDir)

	copyFiles(filepath.Join(cloneDir, "AUTHORS"), outputDir)
	copyFiles(filepath.Join(cloneDir, "LICENSE"), outputDir)
	copyFiles(filepath.Join(cloneDir, "go.mod"), outputDir)
	copyFiles(filepath.Join(cloneDir, "go.sum"), outputDir)

	netOutputPath := filepath.Join(outputDir, "net")
	os.MkdirAll(netOutputPath, 0777)
	copyFiles(filepath.Join(cloneDir, "net", "netaddr"), netOutputPath)
	copyFiles(filepath.Join(cloneDir, "net", "dns"), netOutputPath)
	removeFiles(filepath.Join(outputDir, "net", "dns"), "resolver", "publicdns",
		"manager.go", "manager_test.go", "manager_linux_test.go", "manager_tcp_test.go", "manager_windows_test.go",
		"config.go",
	)
	os.WriteFile(filepath.Join(outputDir, "net", "dns", "manager.go"), managerFile, 0666)
	recursiveReplaceInFiles(filepath.Join(outputDir, "net", "dns", "manager_linux.go"), `"github.com/anywherelan/ts-dns/util/clientmetric"`, "")
	recursiveReplaceInFiles(filepath.Join(outputDir, "net", "dns", "manager_linux.go"), `"strings"`, "")
	recursiveReplaceInFiles(filepath.Join(outputDir, "net", "dns", "manager_linux.go"),
		"publishOnce.Do(func() {\n\t\tsanitizedMode := strings.ReplaceAll(mode, \"-\", \"_\")\n\t\tm := clientmetric.NewGauge(fmt.Sprintf(\"dns_manager_linux_mode_%s\", sanitizedMode))\n\t\tm.Set(1)\n\t})",
		"",
	)

	netInterfacesOutputPath := filepath.Join(netOutputPath, "interfaces")
	os.MkdirAll(netInterfacesOutputPath, 0777)
	os.WriteFile(filepath.Join(netInterfacesOutputPath, "interfaces.go"), interfacesFile, 0666)

	utilOutputPath := filepath.Join(outputDir, "util")
	os.MkdirAll(utilOutputPath, 0777)
	copyFiles(filepath.Join(cloneDir, "util", "dnsname"), utilOutputPath)
	copyFiles(filepath.Join(cloneDir, "util", "cmpver"), utilOutputPath)
	copyFiles(filepath.Join(cloneDir, "util", "winutil"), utilOutputPath)
	copyFiles(filepath.Join(cloneDir, "util", "mak"), utilOutputPath)
	copyFiles(filepath.Join(cloneDir, "util", "lineread"), utilOutputPath)

	copyFiles(filepath.Join(cloneDir, "atomicfile"), outputDir)

	versionOutputPath := filepath.Join(outputDir, "version")
	os.MkdirAll(versionOutputPath, 0777)
	copyFiles(filepath.Join(cloneDir, "version", "distro"), versionOutputPath)

	logtailOutputPath := filepath.Join(outputDir, "logtail")
	os.MkdirAll(logtailOutputPath, 0777)
	copyFiles(filepath.Join(cloneDir, "logtail", "backoff"), logtailOutputPath)

	typesOutputPath := filepath.Join(outputDir, "types")
	os.MkdirAll(typesOutputPath, 0777)
	copyFiles(filepath.Join(cloneDir, "types", "logger"), typesOutputPath)
	removeFiles(filepath.Join(typesOutputPath, "logger"), "logger_test.go")
	copyFiles(filepath.Join(cloneDir, "types", "lazy"), typesOutputPath)

	envknobOutputPath := filepath.Join(outputDir, "envknob")
	os.MkdirAll(envknobOutputPath, 0777)
	os.WriteFile(filepath.Join(envknobOutputPath, "envknob.go"), envknobFile, 0666)

	healthOutputPath := filepath.Join(outputDir, "health")
	os.MkdirAll(healthOutputPath, 0777)
	os.WriteFile(filepath.Join(healthOutputPath, "health.go"), healthFile, 0666)

	recursiveReplaceInFiles(outputDir, `"tailscale.com/`, `"github.com/anywherelan/ts-dns/`)
	recursiveReplaceInFiles(filepath.Join(outputDir, "go.mod"), "module tailscale.com", "module github.com/anywherelan/ts-dns")

	os.Chdir(outputDir)
	runCommand("go", "mod", "tidy")
	runCommand("go", "build", "./...")
	runCommand("go", "fmt", "./...")
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
