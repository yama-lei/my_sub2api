# deploy/

| 文件 | 用途 |
|---|---|
| `deploy.sh` + `deploy.yaml` | `make deploy`：构建已提交的源码镜像，推送到远程服务器并更新应用容器。模板见 `deploy.example.yaml`，`deploy.yaml` 不入库 |
| `docker-compose.dev.yml` | 本地 docker 开发（`make docker-dev`），读取 `deploy/.env`，使用命名卷 |
| `.env.example` | 本地开发环境变量模板：`cp .env.example .env` 后填写 |
| `config.example.yaml` | 应用配置参考 |
| `docker-entrypoint.sh` | 镜像入口，被根目录 `Dockerfile` 使用 |

所有命令都在仓库根目录用 `make` 执行，见根目录 `Makefile`，命令说明见 `DEV_GUIDE.md`。

## 服务器前提

`make deploy` 只替换服务器上 `sub2api` 容器的镜像，**不会**创建服务器环境。远程 `remote_dir` 下必须已有
`docker-compose.yml`、`.env`，且 postgres / redis 容器已在运行。首次搭建服务器时请自行准备这两个文件
（可参考 `docker-compose.dev.yml` 与 `.env.example`）。
