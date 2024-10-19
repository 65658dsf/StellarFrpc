# StellarFrpc 介绍

## 概述

StellarFrpc 是一个基于 [Frp](https://github.com/fatedier/frp) 魔改后的版本，它是一个高性能的反向代理应用，支持多种协议，包括 TCP、UDP、HTTP、HTTPS 等。StellarFrpc 旨在简化部署和使用过程，通过命令行即可一键启动服务。

## 特点

- **一键启动**：通过命令行参数即可快速启动服务，无需复杂的配置文件。
- **高可用性**：支持集群模式，提高服务的可用性和稳定性。
- **易用性**：友好的命令行界面，简化操作流程。

## 安装

StellarFrpc 可以通过多种方式安装，以下是通过docker安装的示例：
[教程链接](https://docs.stellarfrp.top/user/docker.html)
## 启动服务

使用以下命令启动 StellarFrpc 服务：

```bash
StellarFrpc -u 用户token -t 隧道名
```

## 常见问题

- **Q: 如何更新 StellarFrpc？**
  - A: 请访问 [StellarFrpc控制台](https://console.stellarfrp.top/) 下载最新版本并替换旧版本。


## 贡献

我们欢迎任何形式的贡献，包括代码、文档、翻译等。如果你对 StellarFrp 感兴趣，欢迎加入我们的开发团队。
