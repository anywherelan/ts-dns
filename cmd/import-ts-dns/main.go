package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed files_replacement
var replacementFiles embed.FS

// replacements maps a destination path in the output to a stub file from files_replacement.
var replacements = map[string]string{
	"net/dns/manager.go":                     "manager.go",
	"envknob/envknob.go":                     "envknob.go",
	"control/controlknobs/controlknobs.go":   "controlknobs.go",
	"health/health.go":                       "health.go",
	"feature/feature.go":                     "feature.go",
	"hostinfo/hostinfo.go":                   "hostinfo.go",
	"util/eventbus/eventbus.go":              "eventbus.go",
	"feature/buildfeatures/buildfeatures.go": "buildfeatures.go",
}

// upstreamDirs are copied as is from upstream.
// Only packages reachable from non-test code of net/dns are needed (check with `go list -deps`),
// plus util/must which is used by net/dns tests.
var upstreamDirs = []string{
	"net/netaddr",
	"net/tsaddr",
	"net/dns",

	"util/dnsname",
	"util/cmpver",
	"util/winutil",
	"util/mak",
	"util/lineiter",
	"util/set",
	"util/ctxkey",
	"util/must",
	"util/vizerror",
	"util/testenv",
	"util/backoff",
	"util/coalescedop",
	"util/syspolicy/pkey",
	"util/syspolicy/ptype",
	"util/syspolicy/policyclient",

	"atomicfile",
	"tstime",
	"syncs",
	"version/distro",
	// not imported by net/dns: the user imports it to enable watching /etc/resolv.conf in direct mode on linux
	"feature/linuxdnsfight",

	"types/logger",
	"types/lazy",
	"types/views",
	"types/dnstype",
	"types/structs",
	"types/result",
}

// testsKeptIn are the only dirs where upstream tests are kept.
// Other packages are copied verbatim and tested upstream, their tests would only pull extra packages and deps.
var testsKeptIn = []string{"net/dns"}

// removePaths are removed after copying upstream dirs.
var removePaths = []string{
	// dns manager and resolver are tailscale specific, we only need OSConfigurator
	"net/dns/resolver",
	"net/dns/publicdns",
	"net/dns/recursive",
	"net/dns/manager.go",
	"net/dns/manager_test.go",
	"net/dns/manager_linux_test.go",
	"net/dns/manager_tcp_test.go",
	"net/dns/manager_windows_test.go",
	"net/dns/manager_apple.go",
	"net/dns/manager_apple_test.go",
	"net/dns/manager_nonapple.go",
	"net/dns/negative_cache_test.go",
	"net/dns/config.go",
	"net/dns/dns_clone.go",
	"net/dns/dns_view.go",
	// tests depending on tstest or other packages we don't import
	"net/dns/direct_linux_test.go",
	"net/dns/osconfig_test.go",
	"net/dns/hosts_option_windows_test.go",

	"util/winutil/authenticode",
	"util/winutil/conpty",
	"util/winutil/policy",
	"util/winutil/s4u",

	// not used by tstime itself, only by other upstream packages
	"tstime/mono",
	"tstime/rate",

	// depends on util/syspolicy/setting, policy_snapshot_omit.go is used instead
	"util/syspolicy/policyclient/policy_snapshot.go",
}

// generatedPaths are top-level paths in the repo produced by this script.
// They are removed before copying the new output, so files deleted upstream don't stay around.
var generatedPaths = []string{
	"atomicfile", "control", "envknob", "feature", "health", "hostinfo", "logtail", "net",
	"syncs", "tstime", "types", "util", "version", "AUTHORS", "LICENSE", "go.mod", "go.sum",
}

// buildOSes are checked to compile before the output is copied to the repo.
var buildOSes = []string{"linux", "windows", "darwin", "freebsd"}

const (
	upstreamRepo = "git@github.com:tailscale/tailscale.git"
	cloneDir     = "temp-clone-dir"
	outputDir    = "temp-output-dir"
)

func main() {
	keepClone := flag.Bool("keep", false, "keep "+cloneDir+" after run and reuse it on the next run instead of cloning again")
	flag.Parse()

	if _, err := os.Stat(cloneDir); err == nil && *keepClone {
		fmt.Println("reusing existing", cloneDir)
	} else {
		os.RemoveAll(cloneDir)
		mustRunCommand("git", "clone", "--depth", "1", upstreamRepo, cloneDir)
	}
	if !*keepClone {
		defer os.RemoveAll(cloneDir)
	}
	os.RemoveAll(outputDir)
	os.Mkdir(outputDir, 0777)
	defer os.RemoveAll(outputDir)

	commit, _ := exec.Command("git", "-C", cloneDir, "rev-parse", "--short=8", "HEAD").Output()
	fmt.Printf("upstream commit %s", commit)

	for _, file := range []string{"LICENSE", "go.mod", "go.sum"} {
		copyUpstream(file)
	}
	for _, dir := range upstreamDirs {
		copyUpstream(dir)
	}
	for _, path := range removePaths {
		os.RemoveAll(filepath.Join(outputDir, path))
	}
	removeTests(outputDir, testsKeptIn)
	for dst, src := range replacements {
		data, err := replacementFiles.ReadFile("files_replacement/" + src)
		if err != nil {
			panic(err)
		}
		dst = filepath.Join(outputDir, dst)
		os.MkdirAll(filepath.Dir(dst), 0777)
		if err := os.WriteFile(dst, data, 0666); err != nil {
			panic(err)
		}
	}

	managerLinux := filepath.Join(outputDir, "net", "dns", "manager_linux.go")
	mustReplaceInFile(managerLinux, `"tailscale.com/util/clientmetric"`, "")
	mustReplaceInFile(managerLinux,
		"publishOnce.Do(func() {\n\t\tsanitizedMode := strings.ReplaceAll(mode, \"-\", \"_\")\n\t\tm := clientmetric.NewGauge(fmt.Sprintf(\"dns_manager_linux_mode_%s\", sanitizedMode))\n\t\tm.Set(1)\n\t})",
		"",
	)
	mustReplaceInFile(filepath.Join(outputDir, "util", "syspolicy", "policyclient", "policy_snapshot_omit.go"), "//go:build ts_omit_syspolicy\n\n", "")

	recursiveReplaceInFiles(outputDir, `"tailscale.com/`, `"github.com/anywherelan/ts-dns/`)
	goMod := filepath.Join(outputDir, "go.mod")
	mustReplaceInFile(goMod, "module tailscale.com", "module github.com/anywherelan/ts-dns")
	// upstream tool deps (linters etc) would otherwise be kept in go.mod by go mod tidy
	goModData, _ := os.ReadFile(goMod)
	goModData = regexp.MustCompile(`(?ms)^tool \(.*?^\)\n?|^tool .*\n`).ReplaceAll(goModData, nil)
	os.WriteFile(goMod, goModData, 0666)

	os.Chdir(outputDir)
	runCommand("go", "mod", "tidy")
	runCommand("go", "fmt", "./...")
	buildOK := true
	check := func(command ...string) {
		if err := runCommand(command[0], command[1:]...); err != nil {
			fmt.Printf("check failed: %v\n", command)
			buildOK = false
		}
	}
	for _, goos := range buildOSes {
		check("env", "GOOS="+goos, "go", "build", "./...")
		// also compiles net/dns tests for each OS, the replacement stubs are used there.
		// Not ./... because upstream code in util/winutil has a vet warning
		check("env", "GOOS="+goos, "go", "vet", "./net/dns/...")
	}
	check("go", "vet", "./...")
	check("go", "test", "-count=1", "./...")
	os.Chdir("..")
	if !buildOK {
		fmt.Println("not copying output. Fix the import script and run again (use -keep to reuse the clone)")
		os.Exit(1)
	}

	for _, path := range generatedPaths {
		os.RemoveAll(path)
	}
	dirEntries, err := os.ReadDir(outputDir)
	if err != nil {
		panic(err)
	}
	for _, entry := range dirEntries {
		mustRunCommand("cp", "-r", filepath.Join(outputDir, entry.Name()), ".")
	}
	fmt.Printf("synced with upstream commit %s", commit)
}

func copyUpstream(path string) {
	dst := filepath.Join(outputDir, path)
	os.MkdirAll(filepath.Dir(dst), 0777)
	mustRunCommand("cp", "-r", filepath.Join(cloneDir, path), dst)
}

// removeTests removes *_test.go files and testdata dirs everywhere in root except in keepIn dirs (and their subdirs).
func removeTests(root string, keepIn []string) {
	var toRemove []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for _, dir := range keepIn {
			if rel == dir {
				return filepath.SkipDir
			}
		}
		if (d.IsDir() && d.Name() == "testdata") || (!d.IsDir() && strings.HasSuffix(d.Name(), "_test.go")) {
			toRemove = append(toRemove, path)
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	for _, path := range toRemove {
		os.RemoveAll(path)
	}
}

func mustReplaceInFile(path, from, to string) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if !bytes.Contains(data, []byte(from)) {
		panic(fmt.Sprintf("%s: pattern not found, upstream changed? %q", path, from))
	}
	info, _ := os.Stat(path)
	os.WriteFile(path, bytes.ReplaceAll(data, []byte(from), []byte(to)), info.Mode())
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

func mustRunCommand(command string, params ...string) {
	if err := runCommand(command, params...); err != nil {
		panic(fmt.Sprintf("%s %v: %v", command, params, err))
	}
}

func runCommand(command string, params ...string) error {
	cmd := exec.Command(command, params...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
