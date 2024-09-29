package main

/*
本程序用于检查给定进程ID (PID) 是否与主机共享网络命名空间。

主要功能：
1. 接受一个进程ID作为命令行参数。
2. 获取主机（PID 1）的网络命名空间。
3. 获取目标进程的网络命名空间。
4. 比较两个网络命名空间是否相同。
5. 输出结果，说明目标进程是否与主机共享网络命名空间。

使用方法：
go run check_network_namespace.go <PID>

注意事项：
- 本程序需要在Linux环境下运行。
- 需要root权限或足够的权限来访问进程的网络命名空间。
- 程序使用github.com/vishvananda/netns库来处理网络命名空间操作。

此程序对于理解容器化环境中进程的网络隔离状态非常有用，
可用于调试、安全审计和系统管理等场景。
*/

import (
	"fmt"
	"os"
	"strconv"

	"github.com/vishvananda/netns"
)

func checkNetworkNamespace(pid int) (bool, error) {
	// 获取宿主机（PID 1）的网络命名空间
	hostNS, err := netns.GetFromPath("/proc/1/ns/net")
	if err != nil {
		return false, fmt.Errorf("failed to get host network namespace: %v", err)
	}
	defer hostNS.Close()

	// 获取目标进程的网络命名空间
	targetNS, err := netns.GetFromPid(pid)
	if err != nil {
		return false, fmt.Errorf("failed to get target process network namespace: %v", err)
	}
	defer targetNS.Close()

	// 比较两个网络命名空间
	return hostNS.Equal(targetNS), nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run check_network_namespace.go <PID>")
		os.Exit(1)
	}

	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("Invalid PID: %v\n", err)
		os.Exit(1)
	}

	shared, err := checkNetworkNamespace(pid)
	if err != nil {
		fmt.Printf("Error checking network namespace: %v\n", err)
		os.Exit(1)
	}

	if shared {
		fmt.Printf("Process with PID %d shares the host's network namespace.\n", pid)
	} else {
		fmt.Printf("Process with PID %d has its own network namespace.\n", pid)
	}
}
