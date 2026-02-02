// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016 Datadog, Inc.

//go:build linux

package internal

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadContainerID(t *testing.T) {
	for in, out := range map[string]string{
		`other_line
10:hugetlb:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
9:cpuset:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
8:pids:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
7:freezer:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
6:cpu,cpuacct:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
5:perf_event:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
4:blkio:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
3:devices:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
2:net_cls,net_prio:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa`: "8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa",
		"10:hugetlb:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa": "8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa",
		"10:hugetlb:/kubepods": "",
		"11:hugetlb:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da": "432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da",
		"1:name=systemd:/docker/34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376":                               "34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376",
		"1:name=systemd:/uuid/34dc0b5e-626f-2c5c-4c51-70e34b10e765":                                                             "34dc0b5e-626f-2c5c-4c51-70e34b10e765",
		"1:name=systemd:/ecs/34dc0b5e626f2c5c4c5170e34b10e765-1234567890":                                                       "34dc0b5e626f2c5c4c5170e34b10e765-1234567890",
		"1:name=systemd:/docker/34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376.scope":                         "34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376",
		`1:name=systemd:/nope
2:pids:/docker/34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376
3:cpu:/invalid`: "34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376",
		`other_line
12:memory:/system.slice/garden.service/garden/6f265890-5165-7fab-6b52-18d1
11:rdma:/
10:freezer:/garden/6f265890-5165-7fab-6b52-18d1
9:hugetlb:/garden/6f265890-5165-7fab-6b52-18d1
8:pids:/system.slice/garden.service/garden/6f265890-5165-7fab-6b52-18d1
7:perf_event:/garden/6f265890-5165-7fab-6b52-18d1
6:cpu,cpuacct:/system.slice/garden.service/garden/6f265890-5165-7fab-6b52-18d1
5:net_cls,net_prio:/garden/6f265890-5165-7fab-6b52-18d1
4:cpuset:/garden/6f265890-5165-7fab-6b52-18d1
3:blkio:/system.slice/garden.service/garden/6f265890-5165-7fab-6b52-18d1
2:devices:/system.slice/garden.service/garden/6f265890-5165-7fab-6b52-18d1
1:name=systemd:/system.slice/garden.service/garden/6f265890-5165-7fab-6b52-18d1`: "6f265890-5165-7fab-6b52-18d1",
		"1:name=systemd:/system.slice/garden.service/garden/6f265890-5165-7fab-6b52-18d1": "6f265890-5165-7fab-6b52-18d1",
	} {
		id := parseContainerID(strings.NewReader(in))
		if id != out {
			t.Fatalf("%q -> %q: %q", in, out, id)
		}
	}
}

func TestReadContainerIDFromCgroup(t *testing.T) {
	cid := "8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa"
	cgroupContents := "10:hugetlb:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/" + cid

	tmpFile, err := os.CreateTemp(os.TempDir(), "fake-cgroup-")
	if err != nil {
		t.Fatalf("failed to create fake cgroup file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	_, err = io.WriteString(tmpFile, cgroupContents)
	if err != nil {
		t.Fatalf("failed writing to fake cgroup file: %v", err)
	}
	err = tmpFile.Close()
	if err != nil {
		t.Fatalf("failed closing fake cgroup file: %v", err)
	}

	actualCID := readContainerID(tmpFile.Name())
	assert.Equal(t, cid, actualCID)
}

func TestReadEntityIDPrioritizeCID(t *testing.T) {
	// reset cid after test
	defer func(cid string) { containerID = cid }(containerID)

	containerID = "fakeContainerID"
	eid := readEntityID("", "", true)
	assert.Equal(t, "ci-fakeContainerID", eid)
}

func TestReadEntityIDFallbackOnInode(t *testing.T) {
	// reset cid after test
	defer func(cid string) { containerID = cid }(containerID)
	containerID = ""

	sysFsCgroupPath := path.Join(os.TempDir(), "sysfscgroup")
	groupControllerPath := path.Join(sysFsCgroupPath, "mynode")
	err := os.MkdirAll(groupControllerPath, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(groupControllerPath)

	stat, err := os.Stat(groupControllerPath)
	require.NoError(t, err)
	expectedInode := fmt.Sprintf("in-%d", stat.Sys().(*syscall.Stat_t).Ino)

	procSelfCgroup, err := ioutil.TempFile("", "procselfcgroup")
	require.NoError(t, err)
	defer os.Remove(procSelfCgroup.Name())
	_, err = procSelfCgroup.WriteString("0::/mynode")
	require.NoError(t, err)
	err = procSelfCgroup.Close()
	require.NoError(t, err)

	eid := readEntityID(sysFsCgroupPath, procSelfCgroup.Name(), false)
	assert.Equal(t, expectedInode, eid)

	emptyEid := readEntityID(sysFsCgroupPath, procSelfCgroup.Name(), true)
	assert.Equal(t, "", emptyEid)
}

func TestParsegroupControllerPath(t *testing.T) {
	// Test cases
	cases := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "cgroup2 normal case",
			content:  `0::/`,
			expected: map[string]string{"": "/"},
		},
		{
			name: "hybrid",
			content: `other_line
0::/
1:memory:/docker/abc123`,
			expected: map[string]string{
				"":       "/",
				"memory": "/docker/abc123",
			},
		},
		{
			name: "with other controllers",
			content: `other_line
12:pids:/docker/abc123
11:hugetlb:/docker/abc123
10:net_cls,net_prio:/docker/abc123
0::/docker/abc123
`,
			expected: map[string]string{
				"": "/docker/abc123",
			},
		},
		{
			name:     "no controller",
			content:  "empty",
			expected: map[string]string{},
		},
	}

	// Run test cases
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reader := strings.NewReader(c.content)
			result := parseCgroupNodePath(reader)
			require.Equal(t, c.expected, result)
		})
	}
}

func TestGetCgroupInode(t *testing.T) {
	tests := []struct {
		description           string
		cgroupNodeDir         string
		procSelfCgroupContent string
		expectedResult        string
		controller            string
	}{
		{
			description:           "matching entry in /proc/self/cgroup and /proc/mounts - cgroup2 only",
			cgroupNodeDir:         "system.slice/docker-abcdef0123456789abcdef0123456789.scope",
			procSelfCgroupContent: "0::/system.slice/docker-abcdef0123456789abcdef0123456789.scope\n",
			expectedResult:        "in-%d", // Will be formatted with inode number
		},
		{
			description:   "matching entry in /proc/self/cgroup and /proc/mounts - cgroup/hybrid only",
			cgroupNodeDir: "system.slice/docker-abcdef0123456789abcdef0123456789.scope",
			procSelfCgroupContent: `
3:memory:/system.slice/docker-abcdef0123456789abcdef0123456789.scope
2:net_cls,net_prio:c
1:name=systemd:b
0::a
`,
			expectedResult: "in-%d",
			controller:     cgroupV1BaseController,
		},
		{
			description:   "non memory or empty controller",
			cgroupNodeDir: "system.slice/docker-abcdef0123456789abcdef0123456789.scope",
			procSelfCgroupContent: `
3:cpu:/system.slice/docker-abcdef0123456789abcdef0123456789.scope
2:net_cls,net_prio:c
1:name=systemd:b
0::a
`,
			expectedResult: "",
			controller:     "cpu",
		},
		{
			description:   "path does not exist",
			cgroupNodeDir: "dummy.scope",
			procSelfCgroupContent: `
3:memory:/system.slice/docker-abcdef0123456789abcdef0123456789.scope
2:net_cls,net_prio:c
1:name=systemd:b
0::a
`,
			expectedResult: "",
		},
		{
			description:           "no entry in /proc/self/cgroup",
			cgroupNodeDir:         "system.slice/docker-abcdef0123456789abcdef0123456789.scope",
			procSelfCgroupContent: "nothing",
			expectedResult:        "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			sysFsCgroupPath := path.Join(os.TempDir(), "sysfscgroup")
			groupControllerPath := path.Join(sysFsCgroupPath, tc.controller, tc.cgroupNodeDir)
			err := os.MkdirAll(groupControllerPath, 0755)
			require.NoError(t, err)
			defer os.RemoveAll(groupControllerPath)

			stat, err := os.Stat(groupControllerPath)
			require.NoError(t, err)
			expectedInode := ""
			if tc.expectedResult != "" {
				expectedInode = fmt.Sprintf(tc.expectedResult, stat.Sys().(*syscall.Stat_t).Ino)
			}

			procSelfCgroup, err := ioutil.TempFile("", "procselfcgroup")
			require.NoError(t, err)
			defer os.Remove(procSelfCgroup.Name())
			_, err = procSelfCgroup.WriteString(tc.procSelfCgroupContent)
			require.NoError(t, err)
			err = procSelfCgroup.Close()
			require.NoError(t, err)

			result := getCgroupInode(sysFsCgroupPath, procSelfCgroup.Name())
			require.Equal(t, expectedInode, result)
		})
	}
}

// setupK8sFiles creates temporary files simulating Kubernetes service account
// mounts and returns the k8sAPIConfig pointing to them. Caller must clean up tmpDir.
func setupK8sFiles(t *testing.T, token, namespace, hostname string) (tmpDir string, cfg k8sAPIConfig) {
	t.Helper()
	tmpDir = t.TempDir()

	tokenPath := path.Join(tmpDir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte(token), 0600))

	namespacePath := path.Join(tmpDir, "namespace")
	require.NoError(t, os.WriteFile(namespacePath, []byte(namespace), 0600))

	hostnamePath := path.Join(tmpDir, "hostname")
	require.NoError(t, os.WriteFile(hostnamePath, []byte(hostname), 0600))

	cfg = k8sAPIConfig{
		tokenPath:     tokenPath,
		namespacePath: namespacePath,
		caCertPath:    path.Join(tmpDir, "ca.crt"), // doesn't need to exist for HTTP tests
		hostnamePath:  hostnamePath,
		scheme:        "http",
	}
	return
}

func TestFetchContainerIDFromK8sAPI(t *testing.T) {
	t.Run("docker_runtime_prefix", func(t *testing.T) {
		expectedCID := "abc123def456abc123def456abc123def456abc123def456abc123def456abc1"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Contains(t, r.URL.Path, "/api/v1/namespaces/default/pods/my-pod")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"status":{"containerStatuses":[{"containerID":"docker://%s"}]}}`, expectedCID)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "test-token", "default", "my-pod")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Equal(t, expectedCID, cid)
	})

	t.Run("containerd_runtime_prefix", func(t *testing.T) {
		expectedCID := "abc123def456abc123def456abc123def456abc123def456abc123def456abc1"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, `{"status":{"containerStatuses":[{"containerID":"containerd://%s"}]}}`, expectedCID)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Equal(t, expectedCID, cid)
	})

	t.Run("no_runtime_prefix", func(t *testing.T) {
		expectedCID := "plain-container-id-no-prefix"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, `{"status":{"containerStatuses":[{"containerID":"%s"}]}}`, expectedCID)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Equal(t, expectedCID, cid)
	})

	t.Run("empty_container_statuses", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"status":{"containerStatuses":[]}}`)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("empty_container_id_field", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"status":{"containerStatuses":[{"containerID":""}]}}`)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("api_returns_500", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("api_returns_403_forbidden", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("invalid_json_response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `not valid json`)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("missing_token_file", func(t *testing.T) {
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.tokenPath = "/nonexistent/token"
		cfg.kubeHost = "127.0.0.1"
		cfg.kubePort = "8080"

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("missing_namespace_file", func(t *testing.T) {
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.namespacePath = "/nonexistent/namespace"
		cfg.kubeHost = "127.0.0.1"
		cfg.kubePort = "8080"

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("missing_hostname_file", func(t *testing.T) {
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.hostnamePath = "/nonexistent/hostname"
		cfg.kubeHost = "127.0.0.1"
		cfg.kubePort = "8080"

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("missing_kube_host", func(t *testing.T) {
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = ""
		cfg.kubePort = "443"

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("missing_kube_port", func(t *testing.T) {
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = "10.0.0.1"
		cfg.kubePort = ""

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("unreachable_api_server", func(t *testing.T) {
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = "127.0.0.1"
		cfg.kubePort = "1" // unlikely to have anything listening
		cfg.scheme = "http"
		cfg.client = &http.Client{Timeout: 1e9} // 1 second timeout

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Empty(t, cid)
	})

	t.Run("multiple_containers_returns_first", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"status":{"containerStatuses":[
				{"containerID":"docker://first111111111111111111111111111111111111111111111111111111111111"},
				{"containerID":"docker://second22222222222222222222222222222222222222222222222222222222222"}
			]}}`)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Equal(t, "first111111111111111111111111111111111111111111111111111111111111", cid)
	})

	t.Run("ignores_init_and_ephemeral_containers", func(t *testing.T) {
		expectedCID := "main_app_container_abc123"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// Realistic K8s pod status with init, app, and ephemeral containers
			fmt.Fprintf(w, `{
				"status": {
					"initContainerStatuses": [
						{"name": "init-db", "containerID": "docker://init_should_be_ignored_111"},
						{"name": "init-config", "containerID": "docker://init_should_be_ignored_222"}
					],
					"containerStatuses": [
						{"name": "my-app", "containerID": "containerd://%s"},
						{"name": "sidecar", "containerID": "containerd://sidecar_container_def456"}
					],
					"ephemeralContainerStatuses": [
						{"name": "debugger", "containerID": "docker://ephemeral_should_be_ignored_333"}
					]
				}
			}`, expectedCID)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "token", "ns", "pod")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Equal(t, expectedCID, cid, "should return the first container from containerStatuses, ignoring init and ephemeral containers")
	})

	t.Run("whitespace_in_files_trimmed", func(t *testing.T) {
		expectedCID := "abc123"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
			assert.Contains(t, r.URL.Path, "/api/v1/namespaces/my-ns/pods/my-host")
			fmt.Fprintf(w, `{"status":{"containerStatuses":[{"containerID":"docker://%s"}]}}`, expectedCID)
		}))
		defer srv.Close()

		u, _ := url.Parse(srv.URL)
		_, cfg := setupK8sFiles(t, "  my-token\n", "  my-ns\n", "  my-host\n")
		cfg.kubeHost = u.Hostname()
		cfg.kubePort = u.Port()
		cfg.client = srv.Client()

		cid := fetchContainerIDFromK8sAPI(cfg)
		assert.Equal(t, expectedCID, cid)
	})
}

func TestParseK8sContainerID(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{"docker prefix", `{"status":{"containerStatuses":[{"containerID":"docker://abc123"}]}}`, "abc123"},
		{"containerd prefix", `{"status":{"containerStatuses":[{"containerID":"containerd://def456"}]}}`, "def456"},
		{"cri-o prefix", `{"status":{"containerStatuses":[{"containerID":"cri-o://ghi789"}]}}`, "ghi789"},
		{"no prefix", `{"status":{"containerStatuses":[{"containerID":"plainid"}]}}`, "plainid"},
		{"empty id", `{"status":{"containerStatuses":[{"containerID":""}]}}`, ""},
		{"empty statuses", `{"status":{"containerStatuses":[]}}`, ""},
		{"no statuses field", `{"status":{}}`, ""},
		{"invalid json", `not json`, ""},
		{"empty body", ``, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseK8sContainerID(strings.NewReader(tc.body))
			assert.Equal(t, tc.expected, result)
		})
	}

	// Verify that initContainerStatuses are ignored and only containerStatuses is used.
	t.Run("ignores_init_containers", func(t *testing.T) {
		body := `{
			"status": {
				"initContainerStatuses": [
					{"name": "init-svc", "containerID": "docker://init_container_id_should_be_ignored"}
				],
				"containerStatuses": [
					{"name": "app", "containerID": "docker://main_app_container_id"}
				]
			}
		}`
		result := parseK8sContainerID(strings.NewReader(body))
		assert.Equal(t, "main_app_container_id", result, "should use containerStatuses, not initContainerStatuses")
	})

	// Verify that when only initContainerStatuses exists (no regular containers), we return empty.
	t.Run("only_init_containers_returns_empty", func(t *testing.T) {
		body := `{
			"status": {
				"initContainerStatuses": [
					{"name": "init-svc", "containerID": "docker://init_only_id"}
				]
			}
		}`
		result := parseK8sContainerID(strings.NewReader(body))
		assert.Empty(t, result, "should not use initContainerStatuses when containerStatuses is absent")
	})

	// Verify ephemeralContainerStatuses are also ignored.
	t.Run("ignores_ephemeral_containers", func(t *testing.T) {
		body := `{
			"status": {
				"ephemeralContainerStatuses": [
					{"name": "debug", "containerID": "docker://ephemeral_debug_id"}
				],
				"containerStatuses": [
					{"name": "app", "containerID": "containerd://real_app_id"}
				]
			}
		}`
		result := parseK8sContainerID(strings.NewReader(body))
		assert.Equal(t, "real_app_id", result, "should use containerStatuses, not ephemeralContainerStatuses")
	})
}
