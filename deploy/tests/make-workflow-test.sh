#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

python3 - <<'PY'
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

root = Path.cwd()
env = dict(os.environ, POSTGRES_PASSWORD="workflow-check-only")
render = subprocess.run(
    ["docker", "compose", "--env-file", "/dev/null", "-p", "sub2api-dev",
     "-f", "deploy/docker-compose.dev.yml", "-f", "deploy/docker-compose.dev.override.yml",
     "config", "--format", "json"],
    env=env, check=True, capture_output=True, text=True,
)
services = json.loads(render.stdout)["services"]
assert services["sub2api"]["image"] == "sub2api:local"
for name in ("sub2api", "postgres", "redis"):
    assert all(v["type"] == "volume" for v in services[name]["volumes"]), "NTFS bind mount remains"

with tempfile.TemporaryDirectory() as directory:
    fixture = Path(directory)
    (fixture / "backend/bin").mkdir(parents=True)
    (fixture / "tools").mkdir()
    shutil.copy(root / "Makefile", fixture / "Makefile")
    shutil.copy(root / "backend/Makefile", fixture / "backend/Makefile")
    (fixture / "tools/pnpm").write_text("#!/bin/sh\nprintf 'frontend\\n' >> \"$WORKFLOW_EVENTS\"\n")
    (fixture / "tools/go").write_text('''#!/bin/sh
case "$GOFLAGS" in *-tags=embed*) ;; *) exit 1 ;; esac
printf 'backend\n' >> "$WORKFLOW_EVENTS"
cat > bin/server <<'SERVER'
#!/bin/sh
printf 'run\n' >> "$WORKFLOW_EVENTS"
SERVER
chmod +x bin/server
''')
    for tool in ("pnpm", "go"):
        (fixture / "tools" / tool).chmod(0o755)
    test_env = dict(os.environ, PATH=f"{fixture / 'tools'}:{os.environ['PATH']}",
                    WORKFLOW_EVENTS=str(fixture / "events"))
    subprocess.run(["make", "run", "VERSION=workflow-test"], cwd=fixture,
                   env=test_env, check=True, capture_output=True)
    assert (fixture / "events").read_text().splitlines() == ["frontend", "backend", "run"]

# Exercise application-only updates and failure handling without production access.
remote_program = (root / "deploy/deploy.sh").read_text().split("<<'REMOTE'\n", 1)[1].split("\nREMOTE\n", 1)[0]
original = """services:
  sub2api:
    image: sub2api:old
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
  postgres:
    image: postgres:18-alpine
  redis:
    image: redis:8-alpine
"""
stub = r'''#!/usr/bin/env python3
import json
import os
from pathlib import Path
import sys
args = sys.argv[1:]
events_path = Path(os.environ["DEPLOY_EVENTS"])
with events_path.open("a") as stream:
    stream.write(json.dumps(args) + "\n")
events = [json.loads(line) for line in events_path.read_text().splitlines()]
updated = any(a[0] == "compose" and "up" in a for a in events)
stage = os.environ.get("DEPLOY_FAIL", "")
if args[0] == "inspect":
    if "--format" in args:
        if "Labels" in args[args.index("--format") + 1]:
            print("wrong-project" if stage == "project" else "sub2api-deploy")
        else:
            print("sha256:new" if updated and stage != "wrong-image" else "sha256:old")
    else:
        print("[]")
elif args[0] == "image":
    if stage == "image":
        sys.exit(1)
    if "--format" in args:
        print("sha256:new")
elif args[0] == "compose":
    if "ps" in args:
        service = args[-1]
        if stage != "missing-" + service:
            print(service + "-fixture")
    elif "config" in args and stage == "validation":
        sys.exit(1)
    elif "up" in args:
        # Catch bash -s commands accidentally consuming the remaining script.
        sys.stdin.buffer.read()
        if stage == "startup":
            sys.exit(1)
elif args[0] == "exec":
    if "wget" in args:
        if stage == "health":
            sys.exit(1)
        print('{"status":"ok"}')
    elif "-version" in args and stage == "version":
        sys.exit(1)
'''
failures = ("", "project", "image", "missing-postgres", "missing-redis", "validation",
            "startup", "health", "version", "wrong-image")
for failure in failures:
    with tempfile.TemporaryDirectory() as directory:
        folder = Path(directory)
        (folder / "tools").mkdir()
        (folder / "data").mkdir()
        (folder / "data/config.yaml").write_text("jwt: retained\n")
        credentials = b"POSTGRES_PASSWORD=keep-me\nJWT_SECRET=keep-this-too\n"
        (folder / ".env").write_bytes(credentials)
        (folder / "docker-compose.yml").write_text(original)
        docker_stub = folder / "tools/docker"
        docker_stub.write_text(stub)
        docker_stub.chmod(0o755)
        events_file = folder / "events"
        test_env = dict(os.environ, PATH=f"{folder / 'tools'}:{os.environ['PATH']}",
                        DEPLOY_EVENTS=str(events_file), DEPLOY_FAIL=failure)
        result = subprocess.run(
            ["bash", "-s", "--", str(folder), "sub2api:new", "sub2api-deploy", "sub2api"],
            input=remote_program, env=test_env, capture_output=True, text=True, timeout=15,
        )
        assert (result.returncode == 0) == (failure == ""), (failure, result.stderr)
        assert (folder / ".env").read_bytes() == credentials
        assert (folder / "data/config.yaml").read_text() == "jwt: retained\n"
        assert not (folder / "docker-compose.deploy-pending.yml").exists()
        events = [json.loads(line) for line in events_file.read_text().splitlines()]
        assert not any(a[0] in ("stop", "start", "run") or "--migrate" in a
                       or any("pg_dump" in x or "pg_restore" in x for x in a) for a in events), events
        assert not any(a[0] == "compose" and "run" in a for a in events), events
        ups = [a for a in events if a[0] == "compose" and "up" in a]
        updated = failure in ("", "startup", "health", "version", "wrong-image")
        assert len(ups) == int(updated), events
        expected = original.replace("sub2api:old", "sub2api:new") if updated else original
        assert (folder / "docker-compose.yml").read_text() == expected
        if updated:
            up = ups[0]
            assert "--no-deps" in up and "--wait" in up and up[-1] == "sub2api"
            assert up[up.index("--pull") + 1] == "never"
            records = list((folder / "backups").iterdir())
            assert len(records) == 1
            record = records[0]
            assert record.stat().st_mode & 0o777 == 0o700
            assert (record / "docker-compose.yml").read_text() == original
            assert (record / "container.json").stat().st_mode & 0o777 == 0o600
            assert not (record / "database.dump").exists()
            assert not (record / "files.tar.gz").exists()
        if failure == "":
            assert any("wget" in a for a in events), events
            assert any("-version" in a for a in events), events
            assert "Deployment complete" in result.stdout
        else:
            assert "Deployment complete" not in result.stdout

print("make workflow test passed")
PY
