// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

//go:build none
// +build none

/*
The ci command is called from Continuous Integration scripts.

Usage: go run build/ci.go <command> <command flags/arguments>

Available commands are:

	lint                                                               -- runs certain pre-selected linters
	install    [ -arch architecture ] [ -cc compiler ] [ packages... ] -- builds packages and executables
	test       [ -coverage ] [ packages... ]                           -- runs the tests

	nsis                                                                                        -- creates a Windows NSIS installer

For all commands, -n prevents execution of external programs (dry run mode).
*/
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/cespare/cp"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/internal/build"
	"github.com/theQRL/go-qrl/internal/download"
	"github.com/theQRL/go-qrl/internal/version"
)

var (
	goModules = []string{
		".",
	}

	// Files that end up in the gqrl-alltools*.zip archive.
	allToolsArchiveFiles = []string{
		"COPYING",
		executablePath("abigen"),
		executablePath("qrvm"),
		executablePath("gqrl"),
		executablePath("rlpdump"),
		executablePath("clef"),
	}
)

var GOBIN, _ = filepath.Abs(filepath.Join("build", "bin"))

func executablePath(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(GOBIN, name)
}

func main() {
	log.SetFlags(log.Lshortfile)

	if !common.FileExist(filepath.Join("build", "ci.go")) {
		log.Fatal("this script must be run from the root of the repository")
	}
	if len(os.Args) < 2 {
		log.Fatal("need subcommand as first argument")
	}
	switch os.Args[1] {
	case "install":
		doInstall(os.Args[2:])
	case "test":
		doTest(os.Args[2:])
	case "lint":
		doLint(os.Args[2:])
	case "dockerx":
		doDockerBuildx(os.Args[2:])
	case "nsis":
		doWindowsInstaller(os.Args[2:])
	default:
		log.Fatal("unknown command ", os.Args[1])
	}
}

// Compiling

func doInstall(cmdline []string) {
	var (
		dlgo       = flag.Bool("dlgo", false, "Download Go and build with it")
		arch       = flag.String("arch", "", "Architecture to cross build for")
		cc         = flag.String("cc", "", "C compiler to cross build with")
		staticlink = flag.Bool("static", false, "Create statically-linked executable")
	)
	flag.CommandLine.Parse(cmdline)
	env := build.Env()

	// Configure the toolchain.
	tc := build.GoToolchain{GOARCH: *arch, CC: *cc}
	if *dlgo {
		csdb := download.MustLoadChecksums("build/checksums.txt")
		tc.Root = build.DownloadGo(csdb)
	}
	// Disable CLI markdown doc generation in release builds.
	buildTags := []string{"urfave_cli_no_docs"}

	// Configure the build.
	gobuild := tc.Go("build", buildFlags(env, *staticlink, buildTags)...)

	// Show packages during build.
	gobuild.Args = append(gobuild.Args, "-v")

	// Now we choose what we're even building.
	// Default: collect all 'main' packages in cmd/ and build those.
	packages := flag.Args()
	if len(packages) == 0 {
		packages = build.FindMainPackages(&tc, "./cmd/...")
	}

	// Do the build!
	for _, pkg := range packages {
		args := slices.Clone(gobuild.Args)
		args = append(args, "-o", executablePath(path.Base(pkg)))
		args = append(args, pkg)
		build.MustRun(&exec.Cmd{Path: gobuild.Path, Args: args, Env: gobuild.Env})
	}
}

// buildFlags returns the go tool flags for building.
func buildFlags(env build.Environment, staticLinking bool, buildTags []string) (flags []string) {
	var ld []string
	// See https://github.com/golang/go/issues/33772#issuecomment-528176001
	// We need to set --buildid to the linker here, and also pass --build-id to the
	// cgo-linker further down.
	ld = append(ld, "--buildid=none")
	if env.Commit != "" {
		ld = append(ld, "-X", "github.com/theQRL/go-qrl/internal/version.gitCommit="+env.Commit)
		ld = append(ld, "-X", "github.com/theQRL/go-qrl/internal/version.gitDate="+env.Date)
	}
	// Strip DWARF on darwin. This used to be required for certain things,
	// and there is no downside to this, so we just keep doing it.
	if runtime.GOOS == "darwin" {
		ld = append(ld, "-s")
	}
	if runtime.GOOS == "linux" {
		// Enforce the stacksize to 8M, which is the case on most platforms apart from
		// alpine Linux.
		// See https://sourceware.org/binutils/docs-2.23.1/ld/Options.html#Options
		// regarding the options --build-id=none and --strip-all. It is needed for
		// reproducible builds; removing references to temporary files in C-land, and
		// making build-id reproducibly absent.
		extld := []string{"-Wl,-z,stack-size=0x800000,--build-id=none,--strip-all"}
		if staticLinking {
			extld = append(extld, "-static")
			// Under static linking, use of certain glibc features must be
			// disabled to avoid shared library dependencies.
			buildTags = append(buildTags, "osusergo", "netgo")
		}
		ld = append(ld, "-extldflags", "'"+strings.Join(extld, " ")+"'")
	}
	// TODO(gballet): revisit after the input api has been defined
	if runtime.GOARCH == "wasm" {
		ld = append(ld, "-gcflags=all=-d=softfloat")
	}
	if len(ld) > 0 {
		flags = append(flags, "-ldflags", strings.Join(ld, " "))
	}
	if len(buildTags) > 0 {
		flags = append(flags, "-tags", strings.Join(buildTags, ","))
	}
	// We use -trimpath to avoid leaking local paths into the built executables.
	flags = append(flags, "-trimpath")
	return flags
}

// Running The Tests
//
// "tests" also includes static analysis tools such as vet.

func doTest(cmdline []string) {
	var (
		dlgo     = flag.Bool("dlgo", false, "Download Go and build with it")
		arch     = flag.String("arch", "", "Run tests for given architecture")
		cc       = flag.String("cc", "", "Sets C compiler binary")
		coverage = flag.Bool("coverage", false, "Whether to record code coverage")
		verbose  = flag.Bool("v", false, "Whether to log verbosely")
		race     = flag.Bool("race", false, "Execute the race detector")
		short    = flag.Bool("short", false, "Pass the 'short'-flag to go test")
		threads  = flag.Int("p", 1, "Number of CPU threads to use for testing")
	)
	flag.CommandLine.Parse(cmdline)

	// Load checksums file (needed for dlgo)
	csdb := download.MustLoadChecksums("build/checksums.txt")

	// Configure the toolchain.
	tc := build.GoToolchain{GOARCH: *arch, CC: *cc}
	if *dlgo {
		tc.Root = build.DownloadGo(csdb)
	}

	gotest := tc.Go("test")

	// CI needs a bit more time for the statetests (default 5m).
	gotest.Args = append(gotest.Args, "-timeout=5m")

	// Enable integration-tests
	gotest.Args = append(gotest.Args, "-tags=integrationtests")

	// Test a single package at a time. CI builders are slow
	// and some tests run into timeouts under load.
	gotest.Args = append(gotest.Args, "-p", fmt.Sprintf("%d", *threads))
	if *coverage {
		gotest.Args = append(gotest.Args, "-covermode=atomic", "-cover")
	}
	if *verbose {
		gotest.Args = append(gotest.Args, "-v")
	}
	if *race {
		gotest.Args = append(gotest.Args, "-race")
	}
	if *short {
		gotest.Args = append(gotest.Args, "-short")
	}

	packages := flag.CommandLine.Args()
	if len(packages) > 0 {
		gotest.Args = append(gotest.Args, packages...)
		build.MustRun(gotest)
		return
	}

	// No packages specified, run all tests for all modules.
	gotest.Args = append(gotest.Args, "./...")
	for _, mod := range goModules {
		test := *gotest
		test.Dir = mod
		build.MustRun(&test)
	}
}

// doLint runs golangci-lint on requested packages.
func doLint(cmdline []string) {
	var (
		cachedir = flag.String("cachedir", "./build/cache", "directory for caching golangci-lint binary.")
	)
	flag.CommandLine.Parse(cmdline)

	linter := downloadLinter(*cachedir)
	linter, err := filepath.Abs(linter)
	if err != nil {
		log.Fatal(err)
	}
	config, err := filepath.Abs(".golangci.yml")
	if err != nil {
		log.Fatal(err)
	}

	lflags := []string{"run", "--config", config}
	packages := flag.CommandLine.Args()
	if len(packages) > 0 {
		build.MustRunCommandWithOutput(linter, append(lflags, packages...)...)
	} else {
		// Run for all modules in workspace.
		for _, mod := range goModules {
			args := append(lflags, "./...")
			lintcmd := exec.Command(linter, args...)
			lintcmd.Dir = mod
			build.MustRunWithOutput(lintcmd)
		}
	}
	fmt.Println("You have achieved perfection.")
}

// downloadLinter downloads and unpacks golangci-lint.
func downloadLinter(cachedir string) string {
	csdb := download.MustLoadChecksums("build/checksums.txt")
	version, err := csdb.FindVersion("golangci")
	if err != nil {
		log.Fatal(err)
	}
	arch := runtime.GOARCH
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	if arch == "arm" {
		arch += "v" + os.Getenv("GOARM")
	}
	base := fmt.Sprintf("golangci-lint-%s-%s-%s", version, runtime.GOOS, arch)
	archivePath := filepath.Join(cachedir, base+ext)
	if err := csdb.DownloadFileFromKnownURL(archivePath); err != nil {
		log.Fatal(err)
	}
	if err := build.ExtractArchive(archivePath, cachedir); err != nil {
		log.Fatal(err)
	}
	return filepath.Join(cachedir, base, "golangci-lint")
}

func archiveBasename(arch string, archiveVersion string) string {
	platform := runtime.GOOS + "-" + arch
	if arch == "arm" {
		platform += os.Getenv("GOARM")
	}
	if arch == "android" {
		platform = "android-all"
	}
	if arch == "ios" {
		platform = "ios-all"
	}
	return platform + "-" + archiveVersion
}

// skips archiving for some build configurations.
func maybeSkipArchive(env build.Environment) {
	if env.IsPullRequest {
		log.Printf("skipping archive creation because this is a PR build")
		os.Exit(0)
	}
	if env.Branch != "main" && !strings.HasPrefix(env.Tag, "v0.") {
		log.Printf("skipping archive creation because branch %q, tag %q is not on the inclusion list", env.Branch, env.Tag)
		os.Exit(0)
	}
}

// Builds the docker images and optionally uploads them to Docker Hub.
func doDockerBuildx(cmdline []string) {
	var (
		platform = flag.String("platform", "", `Push a multi-arch docker image for the specified architectures (usually "linux/amd64,linux/arm64")`)
		hubImage = flag.String("hub", "qrledger/go-qrl", `Where to upload the docker image`)
		upload   = flag.Bool("upload", false, `Whether to trigger upload`)
	)
	flag.CommandLine.Parse(cmdline)

	// Skip building and pushing docker images for PR builds
	env := build.Env()
	maybeSkipArchive(env)

	// Retrieve the upload credentials and authenticate
	user := getenvBase64("DOCKERHUB_USERNAME")
	pass := getenvBase64("DOCKERHUB_TOKEN")

	if len(user) > 0 && len(pass) > 0 {
		auther := exec.Command("docker", "login", "-u", string(user), "--password-stdin")
		auther.Stdin = bytes.NewReader(pass)
		build.MustRun(auther)
	}
	// Retrieve the version infos to build and push to the following paths:
	//  - theqrl/gqrl:latest                            - Pushes to the main branch, Gqrl only
	//  - theqrl/gqrl:stable                            - Version tag publish on GitHub, Gqrl only
	//  - theqrl/gqrl:alltools-latest                   - Pushes to the main branch, Gqrl & tools
	//  - theqrl/gqrl:alltools-stable                   - Version tag publish on GitHub, Gqrl & tools
	//  - theqrl/gqrl:release-<major>.<minor>           - Version tag publish on GitHub, Gqrl only
	//  - theqrl/gqrl:alltools-release-<major>.<minor>  - Version tag publish on GitHub, Gqrl & tools
	//  - theqrl/gqrl:v<major>.<minor>.<patch>          - Version tag publish on GitHub, Gqrl only
	//  - theqrl/gqrl:alltools-v<major>.<minor>.<patch> - Version tag publish on GitHub, Gqrl & tools
	var tags []string

	switch {
	case env.Branch == "main":
		tags = []string{"latest"}
	case strings.HasPrefix(env.Tag, "v0."):
		tags = []string{"stable", fmt.Sprintf("release-%v", version.Family), "v" + version.Semantic}
	}
	// Need to create a mult-arch builder
	check := exec.Command("docker", "buildx", "inspect", "multi-arch-builder")
	if check.Run() != nil {
		build.MustRunCommand("docker", "buildx", "create", "--use", "--name", "multi-arch-builder", "--platform", *platform)
	}

	for _, spec := range []struct {
		file string
		base string
	}{
		{file: "Dockerfile", base: fmt.Sprintf("%s:", *hubImage)},
		{file: "Dockerfile.alltools", base: fmt.Sprintf("%s:alltools-", *hubImage)},
	} {
		for _, tag := range tags { // latest, stable etc
			gqrlImage := fmt.Sprintf("%s%s", spec.base, tag)
			cmd := exec.Command("docker", "buildx", "build",
				"--build-arg", "COMMIT="+env.Commit,
				"--build-arg", "VERSION="+version.WithMeta,
				"--build-arg", "BUILDNUM="+env.Buildnum,
				"--tag", gqrlImage,
				"--platform", *platform,
				"--file", spec.file,
			)
			if *upload {
				cmd.Args = append(cmd.Args, "--push")
			}
			cmd.Args = append(cmd.Args, ".")
			build.MustRun(cmd)
		}
	}
}

func getenvBase64(variable string) []byte {
	dec, err := base64.StdEncoding.DecodeString(os.Getenv(variable))
	if err != nil {
		log.Fatal("invalid base64 " + variable)
	}
	return []byte(dec)
}

func makeWorkdir(wdflag string) string {
	var err error
	if wdflag != "" {
		err = os.MkdirAll(wdflag, 0744)
	} else {
		wdflag, err = os.MkdirTemp("", "gqrl-build-")
	}
	if err != nil {
		log.Fatal(err)
	}
	return wdflag
}

// Windows installer
func doWindowsInstaller(cmdline []string) {
	// Parse the flags and make skip installer generation on PRs
	var (
		arch    = flag.String("arch", runtime.GOARCH, "Architecture for cross build packaging")
		workdir = flag.String("workdir", "", `Output directory for packages (uses temp dir if unset)`)
	)
	flag.CommandLine.Parse(cmdline)
	*workdir = makeWorkdir(*workdir)
	env := build.Env()
	maybeSkipArchive(env)

	// Aggregate binaries that are included in the installer
	var (
		devTools []string
		allTools []string
		gqrlTool string
	)
	for _, file := range allToolsArchiveFiles {
		if file == "COPYING" { // license, copied later
			continue
		}
		allTools = append(allTools, filepath.Base(file))
		if filepath.Base(file) == "gqrl.exe" {
			gqrlTool = file
		} else {
			devTools = append(devTools, file)
		}
	}

	// Render NSIS scripts: Installer NSIS contains two installer sections,
	// first section contains the gqrl binary, second section holds the dev tools.
	templateData := map[string]any{
		"License":  "COPYING",
		"Gqrl":     gqrlTool,
		"DevTools": devTools,
	}
	build.Render("build/nsis.gqrl.nsi", filepath.Join(*workdir, "gqrl.nsi"), 0644, nil)
	build.Render("build/nsis.install.nsh", filepath.Join(*workdir, "install.nsh"), 0644, templateData)
	build.Render("build/nsis.uninstall.nsh", filepath.Join(*workdir, "uninstall.nsh"), 0644, allTools)
	build.Render("build/nsis.pathupdate.nsh", filepath.Join(*workdir, "PathUpdate.nsh"), 0644, nil)
	build.Render("build/nsis.envvarupdate.nsh", filepath.Join(*workdir, "EnvVarUpdate.nsh"), 0644, nil)
	if err := cp.CopyFile(filepath.Join(*workdir, "SimpleFC.dll"), "build/nsis.simplefc.dll"); err != nil {
		log.Fatalf("Failed to copy SimpleFC.dll: %v", err)
	}
	if err := cp.CopyFile(filepath.Join(*workdir, "COPYING"), "COPYING"); err != nil {
		log.Fatalf("Failed to copy copyright note: %v", err)
	}
	// Build the installer. This assumes that all the needed files have been previously
	// built (don't mix building and packaging to keep cross compilation complexity to a
	// minimum).
	ver := strings.Split(version.Semantic, ".")
	if env.Commit != "" {
		ver[2] += "-" + env.Commit[:8]
	}
	installer, err := filepath.Abs("gqrl-" + archiveBasename(*arch, version.Archive(env.Commit)) + ".exe")
	if err != nil {
		log.Fatalf("Failed to convert installer file path: %v", err)
	}
	build.MustRunCommand("makensis.exe",
		"/DOUTPUTFILE="+installer,
		"/DMAJORVERSION="+ver[0],
		"/DMINORVERSION="+ver[1],
		"/DBUILDVERSION="+ver[2],
		"/DARCH="+*arch,
		filepath.Join(*workdir, "gqrl.nsi"),
	)
}
