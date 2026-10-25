#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

# 读取 deploy/deploy.yaml（扁平 key: value，仅接受白名单键，不 eval）。
# 可用环境变量 DEPLOY_CONFIG 指定其他配置文件，例如多台服务器。
config_file=${DEPLOY_CONFIG:-$repo_root/deploy/deploy.yaml}
[[ -r $config_file ]] || {
    echo "Missing $config_file. Copy deploy/deploy.example.yaml to deploy/deploy.yaml and edit it." >&2
    exit 1
}
declare -A cfg=([ssh_port]=22 [local_image]=sub2api:local [app_container]=sub2api)
while IFS= read -r line || [[ -n $line ]]; do
    line=${line%%#*}
    [[ $line =~ ^[[:space:]]*([a-z_]+):[[:space:]]*(.*[^[:space:]])?[[:space:]]*$ ]] || continue
    key=${BASH_REMATCH[1]}
    value=${BASH_REMATCH[2]:-}
    value=${value#\"}; value=${value%\"}
    case $key in
        ssh_host|ssh_port|remote_user|remote_dir|project_name|app_container|local_image) cfg[$key]=$value ;;
        *) echo "Unknown key in $config_file: $key" >&2; exit 1 ;;
    esac
done < "$config_file"
for key in ssh_host remote_user remote_dir project_name; do
    [[ -n ${cfg[$key]:-} ]] || { echo "Missing required key in $config_file: $key" >&2; exit 1; }
done
SSH_HOST=${cfg[ssh_host]} SSH_PORT=${cfg[ssh_port]} REMOTE_USER=${cfg[remote_user]}
REMOTE_DIR=${cfg[remote_dir]} PROJECT_NAME=${cfg[project_name]}
APP_CONTAINER=${cfg[app_container]} LOCAL_IMAGE=${cfg[local_image]}

require_clean_worktree() {
    if [[ -n $(git status --porcelain --untracked-files=normal) ]]; then
        echo "Deployment refused: uncommitted changes. Commit or stash all changes, including untracked files." >&2
        git status --short >&2
        exit 1
    fi
}
require_clean_worktree
revision=$(git rev-parse --short=12 HEAD)

[[ "$SSH_HOST" =~ ^[a-zA-Z0-9_@.-]+$ && "$SSH_HOST" != -* ]] || exit 1
[[ "$SSH_PORT" =~ ^[0-9]{1,5}$ ]] && (( 10#$SSH_PORT > 0 && 10#$SSH_PORT < 65536 )) || exit 1
[[ "$REMOTE_USER" =~ ^[a-z_][a-z0-9_-]*$ ]] || exit 1
[[ "$REMOTE_DIR" =~ ^/[a-zA-Z0-9_./-]+$ && "$REMOTE_DIR" != *..* && "$REMOTE_DIR" != / ]] || exit 1
for name in "$PROJECT_NAME" "$APP_CONTAINER"; do
    [[ "$name" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || exit 1
done
ssh_args=(-o BatchMode=yes -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=4 -p "$SSH_PORT")
ssh "${ssh_args[@]}" "$SSH_HOST" "sudo -n -u $REMOTE_USER bash -c 'set -e; cd $REMOTE_DIR; test -r .env; test -r docker-compose.yml; test \"\$(uname -m)\" = x86_64; command -v python3 >/dev/null; command -v flock >/dev/null; docker compose version; docker inspect $APP_CONTAINER >/dev/null'"

build_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
remote_image="sub2api:deploy-$revision-$(date -u +%Y%m%d%H%M%S)"
printf 'Building current source: %s\n' "$revision"
docker build --network host --platform linux/amd64 \
    --build-arg COMMIT="$revision" --build-arg DATE="$build_date" \
    --label "org.opencontainers.image.revision=$revision" \
    -t "$LOCAL_IMAGE" -t "$remote_image" .
require_clean_worktree
[[ $(git rev-parse --short=12 HEAD) = "$revision" ]] || { echo "Deployment refused: HEAD changed during build." >&2; exit 1; }
docker save "$remote_image" | gzip | ssh "${ssh_args[@]}" "$SSH_HOST" 'gzip -dc | docker load'

ssh "${ssh_args[@]}" "$SSH_HOST" sudo -n -u "$REMOTE_USER" bash -s -- "$REMOTE_DIR" "$remote_image" "$PROJECT_NAME" "$APP_CONTAINER" <<'REMOTE'
set -euo pipefail
umask 077
remote_dir=$1
image=$2
project=$3
container=$4
cd "$remote_dir"
exec 9>.deploy.lock
flock -n 9 || { echo 'Another deployment is running' >&2; exit 1; }
compose=(docker compose -p "$project" --env-file .env -f docker-compose.yml)
[[ $(docker inspect --format '{{index .Config.Labels "com.docker.compose.project"}}' "$container") = "$project" ]] || { echo 'Compose project mismatch' >&2; exit 1; }
expected_image=$(docker image inspect --format '{{.Id}}' "$image")
for dependency in postgres redis; do
    [[ -n $("${compose[@]}" ps -q "$dependency" </dev/null) ]] || { echo "Existing $dependency container is required" >&2; exit 1; }
done

pending="$remote_dir/docker-compose.deploy-pending.yml"
updating=false
finish() {
    result=$?
    trap - EXIT
    rm -f "$pending"
    if (( result != 0 )) && $updating; then
        echo "Deployment failed. Inspect: docker logs --tail 100 $container" >&2
        echo 'Startup may have applied schema changes; do not automatically start an older image or restore the database.' >&2
    fi
    exit "$result"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# 只改现有 Compose 的应用镜像，不覆盖服务器的部署配置。
python3 - "$image" "$pending" <<'PY'
from pathlib import Path
import re
import sys

lines = Path("docker-compose.yml").read_text().splitlines(keepends=True)
in_app = False
changed = 0
for index, line in enumerate(lines):
    if re.fullmatch(r"  [a-zA-Z0-9_-]+:\s*", line):
        in_app = line.strip() == "sub2api:"
    if in_app and line.startswith("    image:"):
        lines[index] = "    image: " + sys.argv[1] + "\n"
        changed += 1
if changed != 1:
    raise SystemExit("Expected exactly one sub2api image in existing Compose")
Path(sys.argv[2]).write_text("".join(lines))
PY
docker compose -p "$project" --env-file .env -f "$pending" config --quiet </dev/null

# 仅保留更新前的配置和镜像记录；数据库备份独立于日常镜像更新。
record_dir="$remote_dir/backups/update-$(date -u +%Y%m%d%H%M%S)-$$"
mkdir -p "$record_dir"
cp -p docker-compose.yml "$record_dir/docker-compose.yml"
docker inspect "$container" > "$record_dir/container.json"
old_image=$(docker inspect --format '{{.Image}}' "$container")
rollback_image="sub2api:rollback-$(basename "$record_dir")"
docker tag "$old_image" "$rollback_image"
printf '%s\n' "$rollback_image" > "$record_dir/image.txt"
printf 'Deployment record (not a database backup): %s\n' "$record_dir"

mv "$pending" docker-compose.yml
updating=true
# bash -s 使用 stdin 读取脚本，所有容器命令都隔离 stdin。
"${compose[@]}" up -d --no-deps --pull never --wait --wait-timeout 180 sub2api </dev/null
[[ $(docker inspect --format '{{.Image}}' "$container") = "$expected_image" ]] || { echo 'Running image mismatch' >&2; exit 1; }
docker exec "$container" wget -Y off -q -T 5 -O - http://localhost:8080/health </dev/null
docker exec "$container" /app/sub2api -version </dev/null
printf '\nDeployment complete. Record: %s\nPrevious image: %s\n' "$record_dir" "$rollback_image"
REMOTE

printf 'Deployed %s to %s:%s\n' "$remote_image" "$SSH_HOST" "$REMOTE_DIR"
