#!/usr/bin/env bash
# usage: sync_prometheus.sh <prometheus_config_path> <base64_file_sd_groups>
set -euo pipefail
python3 - "$1" "$2" "${3:-}" <<'PY'
import base64
import json
import os
from pathlib import Path
import re
import sys
import tempfile

config = Path(sys.argv[1])
groups = json.loads(base64.b64decode(sys.argv[2], validate=True))
if not isinstance(groups, list) or not groups:
    raise ValueError('node inventory must be a nonempty array')
for group in groups:
    if not group.get('targets') or not all(group['labels'].get(k) for k in ('cluster', 'node', 'component')):
        raise ValueError('node targets require cluster, node and component')

def atomic(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(dir=path.parent, prefix=path.name+'.')
    try:
        with os.fdopen(fd, 'w') as stream:
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        os.chmod(temporary, 0o644)
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)

text = config.read_text()
# Remove only the legacy generated node scrape job, including on reruns.
text = re.sub(r"(?m)^  - job_name: ['\"]?node['\"]?\s*\n.*?(?=^  - job_name:|\Z)", '', text, flags=re.S)
if not re.search(r"job_name:\s*['\"]?dingofs-node['\"]?", text):
    text = text.rstrip() + '''

  - job_name: 'dingofs-node'
    file_sd_configs:
    - files: ['/prometheus/node-targets.json']
      refresh_interval: 15s
'''
atomic(config.parent/'data/node-targets.json', json.dumps(groups, indent=2)+'\n')
discovery = [{'targets':['127.0.0.1:19100'], 'labels': {
    'cluster':groups[0]['labels']['cluster'], 'component':'discovery',
    'node':sys.argv[3] or groups[0]['labels']['node']}}]
atomic(config.parent/'data/discovery-targets.json', json.dumps(discovery, indent=2)+'\n')
atomic(config, text)
PY
