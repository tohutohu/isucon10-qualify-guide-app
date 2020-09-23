#!/bin/bash -xe

HOSTNAME=$(hostname)
BRANCH=${1:-master}

cd $(dirname $0)/..
git fetch
git reset --hard origin/$BRANCH
sudo rsync -rv $HOSTNAME/root/ /
cd webapp/go
make
sudo systemctl daemon-reload
sudo bash -c ":>/var/log/nginx/access.log"
sudo bash -c ":>/var/log/mysql/slow.log"

sudo systemctl restart mysql
sudo systemctl restart nginx
sudo systemctl restart isuumo.go
