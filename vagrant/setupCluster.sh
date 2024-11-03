#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

CURRENT_FILENAME=`basename $0`
CURRENT_DIR_PATH=$(cd `dirname $0`; pwd)

# 定义镜像变量
VAGRANT_IMAGE_K8S=${VAGRANT_IMAGE_K8S:-"alvistack/kubernetes-1.30"}
VAGRANT_IMAGE_UBUNTU=${VAGRANT_IMAGE_UBUNTU:-"alvistack/ubuntu-24.04"}
# 定义资源变量
VM_MEMORY=${VM_MEMORY:-$((${VM_MEMORY:-1024}*8))}
VM_CPUS=${VM_CPUS:-"4"}

SKIP_KUBE_PROXY=${SKIP_KUBE_PROXY:-""}

# HOST PORT MAPPING
HOSTPORT_API_SERVER=${HOSTPORT_API_SERVER:-"26443"}
#
HOSTPORT_CONTROLVM_TCP_PORT1=${HOSTPORT_CONTROLVM_TCP_PORT1:-"27000"}
HOSTPORT_WORKERVM_TCP_PORT1=${HOSTPORT_WORKERVM_TCP_PORT1:-"27001"}
VMPORT_K8SVM_TCP_PORT1=${VMPORT_K8SVM_TCP_PORT1:-"27000"}
#
HOSTPORT_HOSTVM_TCP_PORT1=${HOSTPORT_HOSTVM_TCP_PORT1:-"26440"}
VMPORT_HOSTVM_TCP_PORT1=${VMPORT_HOSTVM_TCP_PORT1:-"80"}
#
HOSTPORT_HOSTVM_TCP_PORT2=${HOSTPORT_HOSTVM_TCP_PORT2:-"28000"}
VMPORT_HOSTVM_TCP_PORT2=${VMPORT_HOSTVM_TCP_PORT2:-"8040"}
#
KUBECONFIG_PATH=${KUBECONFIG_PATH:-${CURRENT_DIR_PATH}/config}
#
DEFAULT_ROUTER_TO_HOST="false"

# 检查命令行参数
if [ "$#" -ne 1 ]; then
  echo "Usage: $0 {on|off}"
  exit 1
fi

# 初始化 Vagrantfile
create_vagrantfile() {
  cat <<EOF > Vagrantfile
Vagrant.configure("2") do |config|

  config.vm.provision "shell", privileged: true, run: "once", inline: <<-SHELL
      set -o errexit
      set -o nounset
      set -o pipefail

      # 确保 vagrant 用户具有 sudo 权限
      echo "vagrant ALL=(ALL) NOPASSWD:ALL" | sudo tee /etc/sudoers.d/vagrant
      sudo chmod 0440 /etc/sudoers.d/vagrant
      echo "sudo -i" >> /home/vagrant/.bashrc
      # 生成 SSH 密钥对
      [ -d '/root/.ssh' ] || mkdir /root/.ssh
      cp /home/vagrant/scripts/ssh/* /root/.ssh
      cp /home/vagrant/scripts/ssh/id_rsa.pub  /root/.ssh/authorized_keys
      chmod 0600 /root/.ssh/id_rsa
  SHELL

  # 定义 Kubernetes 主节点虚拟机
  config.vm.define "controlvm" do |k8s|
    k8s.vm.box = "$VAGRANT_IMAGE_K8S"
    k8s.vm.hostname = "controlvm"
    k8s.vm.network "private_network", ip: "192.168.0.10", netmask: "255.255.255.0", ipv6: "fd00::10", ipv6_prefix_length: 64
    k8s.vm.network "forwarded_port", guest: ${VMPORT_K8SVM_TCP_PORT1}, host: ${HOSTPORT_CONTROLVM_TCP_PORT1}
    k8s.vm.network "forwarded_port", guest: 6443, host: ${HOSTPORT_API_SERVER}
    k8s.vm.provider "virtualbox" do |vb|
      vb.memory = "$VM_MEMORY"
      vb.cpus = "$VM_CPUS"
    end
    # 恢复默认的 vagrant 用户 SSH 登录
    # 挂载 scripts 目录
    k8s.vm.synced_folder "./scripts", "/home/vagrant/scripts"
    # 在虚拟机中运行 setUpMaster.sh 脚本
    k8s.vm.provision "shell", inline: <<-SHELL
      set -o errexit
      set -o nounset
      set -o pipefail

      # the image disable ipv6 by default, so reconfigure it 
      sudo sysctl -w net.ipv6.conf.all.disable_ipv6=0
      sudo ip a a 192.168.0.10/24 dev eth1 || true
      sudo ip a a fd00::10/64 dev eth1 || true

      # set up kubernetes master
      chmod +x /home/vagrant/scripts/resetNode.sh
      /home/vagrant/scripts/resetNode.sh 
      chmod +x /home/vagrant/scripts/getImages.sh
      /home/vagrant/scripts/getImages.sh
      chmod +x /home/vagrant/scripts/setUpMaster.sh
      sudo WORKER_JOIN_SCRIPT_PATH=/home/vagrant/scripts/join.sh  SKIP_KUBE_PROXY=${SKIP_KUBE_PROXY} /home/vagrant/scripts/setUpMaster.sh
      sudo /home/vagrant/scripts/setKubelet.sh  eth1
      chmod +x /home/vagrant/scripts/installCalico.sh
      /home/vagrant/scripts/installCalico.sh

      if [ "${DEFAULT_ROUTER_TO_HOST}" == "true" ]; then
        # 删除原有默认路由
        ip route del default || true
        ip -6 route del default || true
        # 设置新的默认路由
        ip route add default via 192.168.0.2
        ip -6 route add default via fd00::2
      fi
    SHELL
  end

  # 定义 Kubernetes 工作节点虚拟机
  config.vm.define "workervm" do |k8s|
    k8s.vm.box = "$VAGRANT_IMAGE_K8S"
    k8s.vm.hostname = "workervm"
    k8s.vm.network "private_network", ip: "192.168.0.11", netmask: "255.255.255.0", ipv6: "fd00::11", ipv6_prefix_length: 64
    k8s.vm.network "forwarded_port", guest: ${VMPORT_K8SVM_TCP_PORT1}, host: ${HOSTPORT_WORKERVM_TCP_PORT1}
    k8s.vm.provider "virtualbox" do |vb|
      vb.memory = "$VM_MEMORY"
      vb.cpus = "$VM_CPUS"
    end
    # 恢复默认的 vagrant 用户 SSH 登录
    # 挂载 scripts 目录
    k8s.vm.synced_folder "./scripts", "/home/vagrant/scripts"
    # 在虚拟机中运行 setUpWorker.sh 脚本
    k8s.vm.provision "shell", inline: <<-SHELL
      set -o errexit
      set -o nounset
      set -o pipefail

      # the image disable ipv6 by default, so reconfigure it 
      sudo sysctl -w net.ipv6.conf.all.disable_ipv6=0
      sudo ip a a 192.168.0.11/24 dev eth1 || true
      sudo ip a a fd00::11/64 dev eth1 || true

      # set up kubernetes worker
      chmod +x /home/vagrant/scripts/resetNode.sh
      /home/vagrant/scripts/resetNode.sh 
      chmod +x /home/vagrant/scripts/getImages.sh
      /home/vagrant/scripts/getImages.sh
      #
      scp -oStrictHostKeyChecking=no -oUserKnownHostsFile=/dev/null 192.168.0.10:/home/vagrant/scripts/pull-calico-image.sh /home/vagrant/scripts/pull-calico-image.sh
      chmod +x /home/vagrant/scripts/pull-calico-image.sh
      sudo /home/vagrant/scripts/pull-calico-image.sh
      #
      scp -oStrictHostKeyChecking=no -oUserKnownHostsFile=/dev/null 192.168.0.10:/home/vagrant/scripts/join.sh /home/vagrant/scripts/join.sh
      chmod +x /home/vagrant/scripts/join.sh
      sudo /home/vagrant/scripts/join.sh 
      sudo /home/vagrant/scripts/setKubelet.sh  eth1
      #
      rm -rf /root/.kube || true
      sudo mkdir /root/.kube || true
      sudo scp -oStrictHostKeyChecking=no -oUserKnownHostsFile=/dev/null 192.168.0.10:/root/.kube/config /root/.kube/config

      if [ "${DEFAULT_ROUTER_TO_HOST}" == "true" ]; then
        # 删除原有默认路由
        ip route del default || true
        ip -6 route del default || true
        # 设置新的默认路由
        ip route add default via 192.168.0.2
        ip -6 route add default via fd00::2
      fi
    SHELL
  end

  # 定义 Ubuntu 虚拟机
  config.vm.define "hostvm" do |ubuntu|
    ubuntu.vm.box = "$VAGRANT_IMAGE_UBUNTU"
    ubuntu.vm.hostname = "hostvm"
    ubuntu.vm.network "private_network", ip: "192.168.0.2", netmask: "255.255.255.0", ipv6: "fd00::2", ipv6_prefix_length: 64
    ubuntu.vm.network "forwarded_port", guest: ${VMPORT_HOSTVM_TCP_PORT1}, host: ${HOSTPORT_HOSTVM_TCP_PORT1}
    ubuntu.vm.network "forwarded_port", guest: ${VMPORT_HOSTVM_TCP_PORT2}, host: ${HOSTPORT_HOSTVM_TCP_PORT2}

    ubuntu.vm.provider "virtualbox" do |vb|
      vb.memory = "$VM_MEMORY"
      vb.cpus = "$VM_CPUS"
    end
    # 恢复默认的 vagrant 用户 SSH 登录
    # 挂载 scripts 目录
    ubuntu.vm.synced_folder "./scripts", "/home/vagrant/scripts"
    # 启用 IP 转发和调整 iptables 规则
    ubuntu.vm.provision "shell", inline: <<-SHELL
      set -o errexit
      set -o nounset
      set -o pipefail

      # the image disable ipv6 by default, so reconfigure it 
      sudo sysctl -w net.ipv6.conf.all.disable_ipv6=0
      sudo ip a a 192.168.0.2/24 dev eth1 || true
      sudo ip a a fd00::2/64 dev eth1 || true

      apt-get update
      apt-get install -y docker.io

      chmod +x /home/vagrant/scripts/router.sh
      /home/vagrant/scripts/router.sh on
    SHELL
  end
end
EOF
}

SetKubeconfig(){
    sshpass -p vagrant scp -oStrictHostKeyChecking=no -oUserKnownHostsFile=/dev/null -P 2222 vagrant@127.0.0.1:/home/vagrant/.kube/config ${KUBECONFIG_PATH}
    cp ${KUBECONFIG_PATH} ${KUBECONFIG_PATH}_old
    sed -i -E 's?server: .*?server: https://127.0.0.1:'${HOSTPORT_API_SERVER}'?' ${KUBECONFIG_PATH}
    export KUBECONFIG=${KUBECONFIG_PATH}
    echo "wait for cluster ready"
    DONE=""
    for i in {1..60}; do
      echo "waiting for cluster ready: ${i}"
      if kubectl get pod -A | sed '1d' | grep -v Running &>/dev/null ; then
        sleep 10
        continue
      fi
      DONE="true"
      break
    done
    if [ -z "$DONE" ] ; then
       echo "k8s cluster is not ready, time out"
       exit 1
    fi

    echo "============================"
    kubectl get pod -A
    echo "============================"

    echo "export KUBECONFIG=${KUBECONFIG_PATH}"
    echo ""
}


# 根据参数执行相应操作
case "$1" in
  on)
    echo "============================================================================"
    echo "start setting up vagrant cluster: $(date)"
    create_vagrantfile
    vagrant up
    SetKubeconfig
    echo "finish setting up vagrant cluster: $(date)"
    echo "============================================================================"
    ;;
  off)
    echo "============================================================================"
    echo "destroy vagrant cluster"
    vagrant destroy -f controlvm workervm hostvm
    echo "============================================================================"
    ;;
  *)
    echo "Invalid command. Use 'on' to start the VMs or 'off' to destroy them."
    exit 1
    ;;
esac
