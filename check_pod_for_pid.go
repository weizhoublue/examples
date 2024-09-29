/*
本程序用于检查给定进程ID (PID) 所属的 Kubernetes Pod 信息。

主要功能：
1. 接受一个进程ID作为命令行参数。
2. 通过分析该进程的 cgroup 信息，获取其所属的 Pod ID 和 Container ID。
3. 如果进程不属于任何 Kubernetes Pod，程序会将其识别为主机进程。
4. 如果进程属于 Kubernetes Pod，程序会连接到 Kubernetes 集群，
   并尝试获取该 Pod 的详细信息，包括 Namespace 和 Pod 名称。
5. 最后，程序会输出进程所属的 Pod 信息，或者在无法找到匹配的 Pod 时输出错误信息。

使用方法：
go run check_pod_for_pid.go <PID>

注意事项：
- 本程序需要在能够访问 Kubernetes 集群的环境中运行。
- 需要正确配置 kubeconfig 文件（默认路径：~/.kube/config）。
- 程序使用正则表达式来解析 cgroup 路径，以适应不同的 Kubernetes 环境。

此程序对于理解容器化环境中进程与 Kubernetes Pod 之间的关系非常有用，
可用于调试、监控和系统管理等场景。
*/

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1" // 修改这行
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run check_pod_for_pid.go <PID>")
		os.Exit(1)
	}

	pid := os.Args[1]
	cgroupPath := fmt.Sprintf("/proc/%s/cgroup", pid)

	podID, containerID, isHostProcess := getPodAndContainerID(cgroupPath)
	if isHostProcess {
		fmt.Printf("进程 %s 是一个主机进程。\n", pid)
		return
	}

	if podID == "" && containerID == "" {
		fmt.Printf("无法从 cgroup 路径获取 Pod ID 或 Container ID：%s\n", cgroupPath)
		return
	}

	if podID == "" {
		fmt.Printf("Process %s is a host process.\n", pid)
		return
	}

	// Set up Kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating Kubernetes client: %v\n", err)
		return
	}

	pod, found := findPodInfo(clientset, podID, containerID)
	if found {
		printPodInfo(pid, pod, containerID)
	} else {
		fmt.Printf("Process %s belongs to a Kubernetes pod, but pod details could not be found.\n", pid)
		fmt.Printf("Pod ID: %s\n", podID)
		fmt.Printf("Container ID: %s\n", containerID)
	}
}

// getPodAndContainerID 从给定的 cgroup 路径中提取 Pod ID 和 Container ID。
//
// 工作原理：
// 1. 打开并读取 cgroup 文件。
// 2. 使用正则表达式查找包含 "kubepods" 的行。
// 3. 解析该行以提取 Pod ID 和 Container ID。
// 4. Pod ID 通常在第四个路径段中，Container ID 在第五个路径段中。
// 5. 使用正则表达式匹配以适应不同的 cgroup 路径格式。
// 6. 将 Pod ID 中的下划线替换为连字符，以匹配 Kubernetes 中的 UID 格式。
//
// 参数：
//   - cgroupPath: cgroup 文件的路径，通常为 "/proc/<PID>/cgroup"
//
// 返回值：
//   - string: Pod ID（如果找到）
//   - string: Container ID（如果找到）
//   - bool: 是否为主机进程（如果找到）
//   - 如果未找到，两个返回值都为空字符串
func getPodAndContainerID(cgroupPath string) (string, string, bool) {
	file, err := os.Open(cgroupPath)
	if err != nil {
		fmt.Printf("打开 cgroup 文件时出错：%v\n", err)
		return "", "", false
	}
	defer file.Close()

	podRegex := regexp.MustCompile(`kubepods-[^-]+-pod([^.]+)\.slice`)
	containerRegex := regexp.MustCompile(`[^-]+-([^.]+)\.scope`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "kubepods") {
			parts := strings.Split(line, "/")
			if len(parts) >= 4 {
				podMatch := podRegex.FindStringSubmatch(parts[3])
				if len(podMatch) == 2 {
					podID := strings.ReplaceAll(podMatch[1], "_", "-")

					if len(parts) >= 5 {
						containerMatch := containerRegex.FindStringSubmatch(parts[4])
						if len(containerMatch) == 2 {
							return podID, containerMatch[1], false
						}
					}
				}
			}
		} else {
			// 检查是否为主机应用
			if isHostProcess(line) {
				return "", "", true
			}
		}
	}

	return "", "", false
}

var hostPatterns := []*regexp.Regexp{
	regexp.MustCompile(`^0::/$`),
	regexp.MustCompile(`^0::/init\.scope$`),
	regexp.MustCompile(`^0::/user\.slice/.*$`),
	regexp.MustCompile(`^0::/system\.slice/.*$`),
}

// isHostProcess 使用正则表达式检查给定的 cgroup 行是否表示主机进程
func isHostProcess(line string) bool {
	for _, pattern := range hostPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

// findPodInfo 在 Kubernetes 集群中查找与给定 Pod ID 或 Container ID 匹配的 Pod。
//
// 工作原理：
// 1. 使用 Kubernetes 客户端列出所有命名空间中的所有 Pod。
// 2. 遍历 Pod 列表，检查每个 Pod 的 UID 是否与给定的 Pod ID 匹配。
// 3. 如果 Pod ID 不匹配，则检查 Pod 中的每个容器 ID 是否与给定的 Container ID 匹配。
// 4. 如果找到匹配的 Pod，返回该 Pod 的信息和 true。
// 5. 如果遍历完所有 Pod 后仍未找到匹配，返回空 Pod 和 false。
//
// 参数：
//   - clientset: Kubernetes 客户端集合
//   - podID: 要查找的 Pod 的 ID
//   - containerID: 要查找的容器的 ID
//
// 返回值：
//   - corev1.Pod: 找到的 Pod 信息（如果未找到则为空 Pod）
//   - bool: 是否找到匹配的 Pod
func findPodInfo(clientset *kubernetes.Clientset, podID, containerID string) (corev1.Pod, bool) {
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing pods: %v\n", err)
		return corev1.Pod{}, false
	}

	for _, pod := range pods.Items {
		if string(pod.UID) == podID {
			return pod, true
		}

		// 检查容器 ID 是否匹配
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if strings.Contains(containerStatus.ContainerID, containerID) {
				return pod, true
			}
		}
	}

	return corev1.Pod{}, false
}

func printPodInfo(pid string, pod corev1.Pod, containerID string) { // 修改这行
	fmt.Printf("Process %s belongs to the following Pod:\n", pid)
	fmt.Printf("Namespace: %s\n", pod.Namespace)
	fmt.Printf("Pod Name: %s\n", pod.Name)
	fmt.Printf("Container ID: %s\n", containerID)
	if pod.Annotations["kubernetes.io/config.mirror"] != "" {
		fmt.Println("This is a static Pod.")
	}
}
