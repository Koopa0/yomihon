package main_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestASecondYomihonHandsItsFolderToTheRunningOneAndVaultsListsBoth(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("a process on Windows cannot be sent an interrupt")
	}
	binary := buildYomihonBinary(t)
	home := t.TempDir()
	env := append(isolatedUserEnv(home), "PATH="+os.Getenv("PATH"))

	first, second := t.TempDir(), t.TempDir()
	for _, dir := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# A vault\n\nHello.\n"), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
			t.Fatal(err)
		}
	}
	start := freeLoopbackPort(t)
	serving := slices.Concat(env, []string{"YOMIHON_PORT=" + strconv.Itoa(start)})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	server := exec.CommandContext(ctx, binary, first) // #nosec G204 -- the binary this test built and a folder it made
	server.Env = serving
	stdout, err := server.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	var waited error
	go func() {
		waited = server.Wait()
		close(exited)
	}()
	t.Cleanup(func() {
		cancel()
		<-exited
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read the address: %v", err)
	}
	firstAddress := strings.TrimSpace(line)
	if want := "http://127.0.0.1:" + strconv.Itoa(start) + "/"; firstAddress != want {
		t.Fatalf("the first folder is at %s, want the starting port, %s", firstAddress, want)
	}
	go func() { _, _ = io.Copy(io.Discard, stdout) }() //nolint:errcheck // the process is stopped by the test

	// A second yomihon, asked for another folder while the first is running,
	// gives the folder to it and ends.
	exit, out, errOut := runYomihonBinary(t, binary, []string{second}, serving)
	secondAddress := strings.TrimSpace(out)
	if exit != 0 || !regexp.MustCompile(`^http://127\.0\.0\.1:\d+/$`).MatchString(secondAddress) || secondAddress == firstAddress {
		t.Fatalf("the second yomihon = exit %d, stdout %q, stderr %q; want 0 and an address of its own", exit, out, errOut)
	}

	exit, out, errOut = runYomihonBinary(t, binary, []string{"vaults", "--format", "json"}, env)
	var listed []struct{ Name, Path, Address string }
	if exit != 0 || json.Unmarshal([]byte(out), &listed) != nil || len(listed) != 2 {
		t.Fatalf("yomihon vaults = exit %d, stdout %q, stderr %q; want both vaults", exit, out, errOut)
	}
	addresses := map[string]bool{}
	for _, l := range listed {
		addresses[l.Address] = true
	}
	if !addresses[firstAddress] || !addresses[secondAddress] {
		t.Errorf("yomihon vaults lists %v, want %s and %s", addresses, firstAddress, secondAddress)
	}
	for _, address := range []string{firstAddress, secondAddress} {
		waitForPage(t, address)
	}

	// Removing a vault takes it out of the list at once and leaves the process
	// serving it until the process ends.
	if exit, _, errOut = runYomihonBinary(t, binary, []string{"vaults", "remove", second}, env); exit != 0 {
		t.Fatalf("yomihon vaults remove = exit %d, stderr %q", exit, errOut)
	}
	_, out, _ = runYomihonBinary(t, binary, []string{"vaults", "--format", "json"}, env)
	if err = json.Unmarshal([]byte(out), &listed); err != nil || len(listed) != 1 {
		t.Errorf("after the removal yomihon vaults lists %q, want one vault", out)
	}
	waitForPage(t, secondAddress)

	// An interrupt ends the process cleanly and frees every port it held.
	if err = server.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
		if waited != nil {
			t.Errorf("an interrupted yomihon ended with %v, want a clean exit", waited)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("yomihon did not stop on an interrupt")
	}
	for _, address := range []string{firstAddress, secondAddress} {
		if _, err = (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", strings.TrimSuffix(strings.TrimPrefix(address, "http://"), "/")); err == nil {
			t.Errorf("%s still answers after yomihon stopped", address)
		}
	}
	// A yomihon that is no longer running leaves the list for the next one.
	_, out, _ = runYomihonBinary(t, binary, []string{"vaults", "--format", "json"}, env)
	if err = json.Unmarshal([]byte(out), &listed); err != nil || len(listed) != 1 || listed[0].Path == "" {
		t.Errorf("after stopping, yomihon vaults lists %q, want the one vault left", out)
	}
}

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	at, err := netip.ParseAddrPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	return int(at.Port())
}

// waitForPage waits until the address answers with a page that is not the one
// that says the vault is being read.
func waitForPage(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		if resp, doErr := (&http.Client{Timeout: 5 * time.Second}).Do(req); doErr == nil {
			body, readErr := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if readErr == nil && closeErr == nil && resp.StatusCode == http.StatusOK && !strings.Contains(string(body), "data-scanning") {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never answered with a page", address)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
