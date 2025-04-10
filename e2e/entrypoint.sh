#!/bin/bash
set -e

SCRIPT_DIR=$(dirname "$(readlink -f "$0")")
mkdir -p /root/.config/atest
mkdir -p /var/data

echo "start to run server"
nohup atest server&

kind=elasticsearch target=https://elasticsearch:9200 atest run -p testing-data-query.yaml

cat /root/.config/atest/stores.yaml
