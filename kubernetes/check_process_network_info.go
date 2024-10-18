/*
本程序用于获取指定进程的网络信息,主要包括IP地址。

主要功能:
1. 接受一个进程ID(PID)作为命令行参数。
2. 可选地接受一个或多个网络接口名称作为附加参数。
3. 切换到目标进程的网络命名空间。
4. 获取指定网络接口(如果提供)或所有接口的IPv4和IPv6地址。
5. 输出获取到的IP地址信息。

使用方法:
go run check_process_network_info.go <PID> [interface1] [interface2] ...

工作原理:
1. 使用netns包切换到目标进程的网络命名空间。
2. 遍历指定的网络接口(或所有接口),获取其IP地址。
3. 将获取到的IP地址分类为IPv4和IPv6。
4. 返回到原始网络命名空间并输出结果。

注意事项:
- 需要root权限才能切换网络命名空间。
- 如果不指定接口名称,将获取所有接口的IP地址。
- 程序会同时获取IPv4和IPv6地址。

此程序对于理解容器化环境中进程的网络配置非常有用,
可用于网络调试、监控和系统管理等场景。
*/

package main

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/vishvananda/netns"
)

type IPAddresses struct {
	IPv4 []net.IP
	IPv6 []net.IP
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check_process_network_info.go <PID> [interface1] [interface2] ...")
		os.Exit(1)
	}

	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("Invalid PID: %v\n", err)
		os.Exit(1)
	}

	interfaceNames := os.Args[2:]

	ips, err := GetContainerIP(pid, interfaceNames)
	if err != nil {
		fmt.Printf("Error getting IP addresses: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Process %d IP addresses:\n", pid)
	fmt.Println("IPv4 addresses:")
	for _, ip := range ips.IPv4 {
		fmt.Println(ip)
	}
	fmt.Println("IPv6 addresses:")
	for _, ip := range ips.IPv6 {
		fmt.Println(ip)
	}
}

func GetContainerIP(pid int, interfaceNames []string) (*IPAddresses, error) {
	// Save current network namespace
	currentNS, err := netns.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get current network namespace: %v", err)
	}
	defer currentNS.Close()

	// Get target process network namespace
	targetNS, err := netns.GetFromPid(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to get target process network namespace: %v", err)
	}
	defer targetNS.Close()

	var allIPs IPAddresses

	// Switch to target network namespace
	err = netns.Set(targetNS)
	if err != nil {
		return nil, fmt.Errorf("failed to switch to target network namespace: %v", err)
	}

	// Get all network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}

	for _, iface := range interfaces {
		// Skip loopback interface
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// If interface names are specified, only process those
		if len(interfaceNames) > 0 && !containStr(interfaceNames, iface.Name) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("failed to get the ip of interface %s: %v", iface.Name, err)
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP

			// Filter out link-local addresses
			if ip.IsLinkLocalUnicast() {
				continue
			}

			if ip.To4() != nil {
				if !containsIP(allIPs.IPv4, ip) {
					allIPs.IPv4 = append(allIPs.IPv4, ip)
				}
			} else {
				if !containsIP(allIPs.IPv6, ip) {
					allIPs.IPv6 = append(allIPs.IPv6, ip)
				}
			}
		}
	}

	// Switch back to original network namespace
	err = netns.Set(currentNS)
	if err != nil {
		return nil, fmt.Errorf("failed to switch back to original network namespace: %v", err)
	}

	if len(allIPs.IPv4) == 0 && len(allIPs.IPv6) == 0 {
		return nil, fmt.Errorf("no valid IP addresses found")
	}

	return &allIPs, nil
}

func containStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsIP(slice []net.IP, ip net.IP) bool {
	for _, a := range slice {
		if a.Equal(ip) {
			return true
		}
	}
	return false
}
