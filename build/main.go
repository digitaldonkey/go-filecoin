package main

import (
	"fmt"
	gobuild "go/build"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var lineBreak = "\n"

func init() {
	log.SetFlags(0)
	if runtime.GOOS == "windows" {
		lineBreak = "\r\n"
	}
}

// run executes a given command on the shell, like
// `run("git status")`
func run(name string) {
	args := strings.Split(name, " ")
	runParts(args...)
}

func runParts(args ...string) {
	name := strings.Join(args, " ")
	cmd := exec.Command(args[0], args[1:]...) // #nosec
	log.Println(name)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err = io.Copy(os.Stderr, stderr); err != nil {
			panic(err)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err = io.Copy(os.Stdout, stdout); err != nil {
			panic(err)
		}
	}()

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	wg.Wait()
	if err := cmd.Wait(); err != nil {
		log.Fatalf("Command '%s' failed: %s\n", name, err)
	}
}

func runCapture(name string) string {
	args := strings.Split(name, " ")
	cmd := exec.Command(args[0], args[1:]...) // #nosec
	log.Println(name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Command '%s' failed: %s\n", name, err)
	}

	return strings.Trim(string(output), lineBreak)
}

// deps installs all dependencies
func deps() {
	log.Println("Installing dependencies...")

	list := []string{
		"git submodule update --init",
		"go get github.com/whyrusleeping/gx",
		"go get github.com/whyrusleeping/gx-go",
		"gx install",
		"gx-go rewrite",
		"go get github.com/alecthomas/gometalinter",
		"gometalinter --install",
		"go get github.com/stretchr/testify",
		"go get github.com/xeipuuv/gojsonschema",
		"go get github.com/ipfs/iptb",
		"go get github.com/docker/docker/api/types",
		"go get github.com/docker/docker/api/types/container",
		"go get github.com/docker/docker/client",
		"go get github.com/docker/docker/pkg/stdcopy",
		"go get github.com/ipsn/go-secp256k1",
		"cargo build --release --all --manifest-path proofs/rust-proofs/Cargo.toml",
	}

	for _, name := range list {
		run(name)
	}
}

// smartdeps avoids fetching from the network
func smartdeps() {
	log.Println("Installing dependencies...")

	// commands we need to run
	cmds := []string{
		"git submodule update --init",
		"gx install",
		"gx-go rewrite",
		"gometalinter --install",
		"cargo build --release --all --manifest-path proofs/rust-proofs/Cargo.toml",
	}
	// packages we need to install
	pkgs := []string{
		"github.com/alecthomas/gometalinter",
		"github.com/docker/docker/api/types",
		"github.com/docker/docker/api/types/container",
		"github.com/docker/docker/client",
		"github.com/docker/docker/pkg/stdcopy",
		"github.com/ipfs/iptb",
		"github.com/stretchr/testify",
		"github.com/whyrusleeping/gx",
		"github.com/whyrusleeping/gx-go",
		"github.com/xeipuuv/gojsonschema",
		"github.com/ipsn/go-secp256k1",
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = gobuild.Default.GOPATH
	}

	gpbin := filepath.Join(gopath, "bin")
	var gopathBinFound bool
	for _, s := range strings.Split(os.Getenv("PATH"), ":") {
		if s == gpbin {
			gopathBinFound = true
		}
	}

	if !gopathBinFound {
		fmt.Println("'$GOPATH/bin' is not in your $PATH.")
		fmt.Println("See https://golang.org/doc/code.html#GOPATH for more information.")
		return
	}

	// if the package exists locally install it, else fetch it
	for _, pkg := range pkgs {
		pkgpath := filepath.Join(gopath, "src", pkg)
		if _, err := os.Stat(pkgpath); os.IsNotExist(err) {
			run(fmt.Sprintf("go get %s", pkg))
		} else {
			run(fmt.Sprintf("go install %s", pkg))
		}
	}

	for _, op := range cmds {
		run(op)
	}
}

// lint runs linting using gometalinter
func lint(packages ...string) {
	if len(packages) == 0 {
		packages = []string{"./..."}
	}

	log.Printf("Linting %s ...\n", strings.Join(packages, " "))

	// Run fast linters batched together
	configs := []string{
		"gometalinter",
		"--skip=sharness",
		"--skip=vendor",
		"--disable-all",
	}

	fastLinters := []string{
		"--enable=vet",
		"--enable=gofmt",
		"--enable=misspell",
		"--enable=goconst",
		"--enable=golint",
		"--enable=errcheck",
		"--min-occurrences=6", // for goconst
	}

	runParts(append(append(configs, fastLinters...), packages...)...)

	slowLinters := []string{
		"--deadline=10m",
		"--enable=unconvert",
		"--enable=gosimple",
		"--enable=megacheck",
		"--enable=varcheck",
		"--enable=structcheck",
		"--enable=deadcode",
	}

	runParts(append(append(configs, slowLinters...), packages...)...)
}

func build() {
	buildFilecoin()
	buildFakecoin()
	buildGengen()
	buildFaucet()
	generateGenesis()
}

func generateGenesis() {
	log.Println("Generating genesis...")
	runParts(
		"./gengen/gengen",
		"--keypath", "fixtures",
		"--out-car", "fixtures/genesis.car",
		"--out-json", "fixtures/gen.json",
		"--config", "./fixtures/setup.json",
	)
}

func buildFakecoin() {
	log.Println("Building go-fakecoin...")
	runParts(
		"go", "build",
		"-o", "tools/go-fakecoin/go-fakecoin",
		"-v",
		"./tools/go-fakecoin",
	)
}

func buildFilecoin() {
	log.Println("Building go-filecoin...")

	commit := runCapture("git log -n 1 --format=%H")

	runParts(
		"go", "build",
		"-ldflags", fmt.Sprintf("-X github.com/filecoin-project/go-filecoin/flags.Commit=%s", commit),
		"-v", "-o", "go-filecoin", ".",
	)
}

func buildGengen() {
	log.Println("Building gengen utils...")

	runParts("go", "build", "-o", "./gengen/gengen", "./gengen")
}

func buildFaucet() {
	log.Println("Building faucet...")

	runParts("go", "build", "-o", "./tools/faucet/faucet", "./tools/faucet/")
}

func install() {
	log.Println("Installing...")

	runParts("go", "install")
}

// test executes tests and passes along all additional arguments to `go test`.
func test(args ...string) {
	log.Println("Testing...")

	run(fmt.Sprintf("go test -parallel 8 ./... %s", strings.Join(args, " ")))
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		log.Fatalf("Missing command")
	}

	cmd := args[0]

	switch cmd {
	case "deps":
		deps()
	case "smartdeps":
		smartdeps()
	case "lint":
		lint(args[1:]...)
	case "build-fakecoin":
		buildFakecoin()
	case "build-filecoin":
		buildFilecoin()
	case "build-gengen":
		buildGengen()
	case "generate-genesis":
		generateGenesis()
	case "build":
		build()
	case "test":
		test(args[1:]...)
	case "install":
		install()
	case "best":
		build()
		test(args[1:]...)
	case "all":
		deps()
		lint()
		build()
		test(args[1:]...)
	default:
		log.Fatalf("Unknown command: %s\n", cmd)
	}
}
