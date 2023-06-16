# Migrator

这是基于[修改后的Apptainer](https://github.com/ContainerLiveMigration/apptainer/tree/feat/criu_bind)的容器迁移原型。

## 编译

```bash
make
```

`make`命令可以直接编译，具体会在`bin`目录下生成两个可执行文件，`server`和`client`。server是服务端，client是客户端。服务端作为后台进程运行在每个节点上，客户端与本地的服务端进程通信，不同节点上的服务端进程再通信，最终实现容器迁移。

## 代码目录

- apptainer，与apptainer交互的代码，包括获取容器实例信息等功能。
- migrator，迁移的核心功能。
- server，服务端
- client，客户端
- util，一些工具函数

## 使用

### 服务端

```bash
./server [--no-shared-fs]
```

默认运行在1234端口。`--no-shared-fs`参数表示没有共享文件系统，后续检查点目录会通过rsync来传输。

### 客户端

```bash
./client [-d|--diskless] <instance name> <target IP>
```

`-d`或`--diskless`表示使用无盘迁移的方案，`<instance name>`是容器实例的名字，`<target IP>`是目标节点的IP地址。

## 迁移流程

### 默认迁移

1. 源节点创建容器检查点

```bash
apptainer checkpoint instance --criu --address <instance name>
```

2. 源节点停止容器实例

```bash
apptainer instance stop <instance name>
```

3. 源节点上如果以`--no-shared-fs`参数运行服务端，会通过rsync命令将检查点目录传输到目标节点。

4. 目标节点上重启容器实例

```bash
apptainer instance start --criu-restart <checkpoint name> <image path> <instance name>
```

### 无盘迁移

1. 源节点上判断检查点目录是否在tmpfs上

2. 如果以`--no-shared-fs`参数运行服务端，会通过rsync命令将检查点目录传输到目标节点。

3. 目标节点以page-server模式启动容器，等待源节点CRIU连接

```bash
apptainer instance start --criu-restart <checkpoint name> --page-server <image path> <instance name>
```

4. 源节点执行dump，会将内存页通过page-server迁移到目标节点，其余进程信息转储特定目录

```bash
apptainer checkpoint instance --criu --page-server --address <target IP> <instance name>
```

5. 如果以`--no-shared-fs`参数运行服务端，从源到目的节点继续同步一些检查点目录下的log文件

6. 从源到目的节点同步位于tmpfs上的检查点目录

7. 目标节点重启程序

```bash
apptainer checkpoint instance --criu --restore <instance name>
```
