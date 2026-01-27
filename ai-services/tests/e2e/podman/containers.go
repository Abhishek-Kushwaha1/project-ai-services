package podman

import (
        "encoding/json"
        "fmt"
        ginkgo "github.com/onsi/ginkgo/v2"
        gomega "github.com/onsi/gomega"
        "github.com/project-ai-services/ai-services/tests/e2e/common"
        "regexp"
        "strconv"
        "strings"
        "testing"
)

func TestPodman(t *testing.T) {
        gomega.RegisterFailHandler(ginkgo.Fail)
        ginkgo.RunSpecs(t, "Pod Status Suite")
}

type PodInspect struct {
        RestartPolicy string `json:"RestartPolicy"`
        Containers    []struct {
                Id   string `json:"Id"`
                Name string `json:"Name"`
        } `json:"Containers"`
}
type ContainerInspect struct {
        State struct {
                RestartCount int `json:"RestartCount"`
        } `json:"State"`
        Config struct {
                Image string `json:"Image"`
        } `json:"Config"`
}

var (
        separatorRe = regexp.MustCompile(`^[\s─-]+$`)
        headerRe    = regexp.MustCompile(`^APPLICATION\s+NAME\s+POD\s+NAME\s+STATUS$`)
        // Matches: [optional app] [pod-name] [status...].
        rowRe = regexp.MustCompile(
                `^\s*(?:(?P<app>\S+)\s+)?(?P<pod>\S+)\s{2,}(?P<status>.+)$`,
        )
        // Extract numeric restart count from status strings like "... Restarts: 0".
        restartRe = regexp.MustCompile(`(?i)restarts?:\s*(?P<count>\d+)`)
)

type PodRow struct {
        AppName      string
        PodName      string
        Status       string
        RestartCount int
}

// parsePodRows parses the output lines from `ai-services application ps` into PodRow structs.
func parsePodRows(lines []string) ([]PodRow, error) {
        rows := make([]PodRow, 0, len(lines))
        for _, raw := range lines {
                line := strings.TrimRight(raw, " \t")
                if line == "" {
                        continue
                }
                if headerRe.MatchString(line) || separatorRe.MatchString(line) {
                        continue
                }
                m := rowRe.FindStringSubmatch(line)
                if m == nil {
                        return nil, fmt.Errorf("unparseable row: %q", line)
                }
                status := strings.TrimSpace(m[rowRe.SubexpIndex("status")])
                restartCount := 0
                if r := restartRe.FindStringSubmatch(status); r != nil {
                        if idx := restartRe.SubexpIndex("count"); idx >= 0 {
                                if v, err := strconv.Atoi(r[idx]); err == nil {
                                        restartCount = v
                                }
                        }
                }
                rows = append(rows, PodRow{
                        AppName:      strings.TrimSpace(m[rowRe.SubexpIndex("app")]),
                        PodName:      strings.TrimSpace(m[rowRe.SubexpIndex("pod")]),
                        Status:       status,
                        RestartCount: restartCount,
                })
        }

        return rows, nil
}

// getRestartCount inspects a pod and its containers and returns the total restart count.
func getRestartCount(podName string) (int, error) {
        podRes, err := common.RunCommand("podman", "pod", "inspect", podName)
        if err != nil {
                return 0, fmt.Errorf("failed to inspect pod %s: %w", podName, err)
        }
        var podData []PodInspect
        if err := json.Unmarshal([]byte(podRes), &podData); err != nil {
                return 0, fmt.Errorf("failed to parse pod inspect for %s: %w", podName, err)
        }
        if len(podData) == 0 {
                return 0, fmt.Errorf("no pod inspect data for %s", podName)
        }
        pod := podData[0]
        if pod.RestartPolicy == "no" {
                return 0, nil
        }
        totalRestarts := 0
        for _, ctr := range pod.Containers {
                ctrRes, err := common.RunCommand("podman", "inspect", ctr.Id)
                if err != nil {
                        return 0, fmt.Errorf("failed to inspect container %s: %w", ctr.Name, err)
                }
                var ctrData []ContainerInspect
                if err := json.Unmarshal([]byte(ctrRes), &ctrData); err != nil {
                        return 0, fmt.Errorf("failed to parse container inspect %s: %w", ctr.Name, err)
                }
                if len(ctrData) == 0 {
                        return 0, fmt.Errorf("no container inspect data for %s", ctr.Name)
                }
                totalRestarts += ctrData[0].State.RestartCount
        }

        return totalRestarts, nil
}

// VerifyContainers checks if application pods are healthy and their restart counts are zero.
func VerifyContainers(appName string) error {
        fmt.Println("[Podman] verifying containers for app:", appName)
        res, _ := common.RunCommand("ai-services", "application", "ps", appName)
        if strings.TrimSpace(res) == "" {
                ginkgo.Skip("No pods found — skipping pod health validation")

                return nil
        }
        lines := strings.Split(strings.TrimSpace(res), "\n")
        rows, err := parsePodRows(lines)
        if err != nil {
                return fmt.Errorf("failed to parse pod rows: %w", err)
        }
        for _, row := range rows {
                ok := strings.HasPrefix(row.Status, "Running (healthy)") || row.Status == "Created"
                if !ok {
                        return fmt.Errorf("pod %s is not healthy (status=%s)", row.PodName, row.Status)
                }
        }
        expectedPodSuffixes := []string{
                "vllm-server",
                "milvus",
                "clean-docs",
                "ingest-docs",
                "chat-bot",
        }
        actualPods := make(map[string]bool)
        for _, row := range rows {
                actualPods[row.PodName] = true
        }
        for _, suffix := range expectedPodSuffixes {
                expectedPodName := appName + "--" + suffix
                gomega.Expect(actualPods).To(gomega.HaveKey(expectedPodName), "expected pod %s to exist", expectedPodName)
                restartCount, err := getRestartCount(expectedPodName)
                gomega.Expect(err).NotTo(gomega.HaveOccurred())
                ginkgo.GinkgoWriter.Printf("[RestartCount] pod=%s restarts=%d\n", expectedPodName, restartCount)
                gomega.Expect(restartCount).To(gomega.BeNumerically("<=", 0),
                        fmt.Sprintf("pod %s restarted %d times", expectedPodName, restartCount))
        }

        return nil
}

// getContainerHostPorts returns host ports mapped for a container by calling `podman port` and parsing output.
func getContainerHostPorts(containerID string) ([]int, error) {
        out, err := common.RunCommand("podman", "port", containerID)
        if err != nil {
                return nil, fmt.Errorf("podman port failed for %s: %w", containerID, err)
        }
        lines := strings.Split(strings.TrimSpace(out), "\n")
        hostPorts := []int{}
        hostPortRe := regexp.MustCompile(`\b\d+\.\d+\.\d+\.\d+:(?P<port>\d+)\b`)
        altRe := regexp.MustCompile(`->\s*(?P<port>\d+)/(tcp|udp)`) // fallback
        for _, l := range lines {
                if l == "" {
                        continue
                }
                if m := hostPortRe.FindStringSubmatch(l); m != nil {
                        idx := hostPortRe.SubexpIndex("port")
                        if idx >= 0 {
                                if p, err := strconv.Atoi(m[idx]); err == nil {
                                        hostPorts = append(hostPorts, p)

                                        continue
                                }
                        }
                }
                if m := altRe.FindStringSubmatch(l); m != nil {
                        idx := altRe.SubexpIndex("port")
                        if idx >= 0 {
                                if p, err := strconv.Atoi(m[idx]); err == nil {
                                        hostPorts = append(hostPorts, p)

                                        continue
                                }
                        }
                }
                // As a last resort, try to find any integer token and use it.
                numRe := regexp.MustCompile(`(\d+)`)
                if m := numRe.FindStringSubmatch(l); m != nil {
                        if p, err := strconv.Atoi(m[1]); err == nil {
                                hostPorts = append(hostPorts, p)
                        }
                }
        }

        return hostPorts, nil
}

// VerifyExposedPorts checks that at least one container exposes each expected host port.
func VerifyExposedPorts(appName string, expectedPorts []int) error {
        res, _ := common.RunCommand("ai-services", "application", "ps", appName)
        if strings.TrimSpace(res) == "" {
                ginkgo.Skip("No pods found — skipping port validation")

                return nil
        }
        lines := strings.Split(strings.TrimSpace(res), "\n")
        rows, err := parsePodRows(lines)
        if err != nil {
                return fmt.Errorf("failed to parse pod rows: %w", err)
        }
        hostPortsFound := make(map[int]bool)
        for _, row := range rows {
                // inspect pod to find container ids
                podRes, err := common.RunCommand("podman", "pod", "inspect", row.PodName)
                if err != nil {
                        return fmt.Errorf("failed to inspect pod %s: %w", row.PodName, err)
                }
                var podData []PodInspect
                if err := json.Unmarshal([]byte(podRes), &podData); err != nil {
                        return fmt.Errorf("failed to parse pod inspect for %s: %w", row.PodName, err)
                }
                if len(podData) == 0 {
                        continue
                }
                for _, ctr := range podData[0].Containers {
                        ports, err := getContainerHostPorts(ctr.Id)
                        if err != nil {
                                // continue to next container rather than fail the whole check
                                continue
                        }
                        for _, p := range ports {
                                hostPortsFound[p] = true
                        }
                }
        }
        for _, ep := range expectedPorts {
                if !hostPortsFound[ep] {
                        return fmt.Errorf("expected host port %d not found on any container for app %s", ep, appName)
                }
        }

        return nil
}