#!/usr/bin/env bash

: <<'COMMENT'
docker compose up spire-server -d

# wait for spire-server to be healthy
docker compose up wait-for-it

# join-token for agent
docker exec spire-server /opt/spire/bin/spire-server token generate -spiffeID spiffe://accuknox.com/1/1 -output json | jq '.value' > join-token.txt
COMMENT

export JOIN_TOKEN=`cat ./join-token.txt`
docker compose up spire-agent --abort-on-container-exit
#docker compose up spire-agent -d

# register local
: <<'COMMENT'
GID=$(id -g)

docker exec spire-server /opt/spire/bin/spire-server entry create -parentID spiffe://accuknox.com/1/1 -spiffeID spiffe://accuknox.com/1/1/pea -selector unix:uid:${UID} -selector unix:gid:${GID} -selector unix:path:"/home/feederservice/bin/feeder-service"

docker exec spire-server /opt/spire/bin/spire-server entry create -parentID spiffe://accuknox.com/1/1 -spiffeID spiffe://accuknox.com/1/1/pea -selector unix:uid:${UID} -selector unix:gid:${GID} -selector unix:path:"/home/pea/main"

docker exec spire-server /opt/spire/bin/spire-server entry create -parentID spiffe://accuknox.com/1/1 -spiffeID spiffe://accuknox.com/1/1/sia -selector unix:uid:${UID} -selector unix:gid:${GID} -selector unix:path:"/usr/bin/sia"
COMMENT

# docker attestator plugin based registration
: <<'COMMENT'
export JOIN_TOKEN=`cat ./join-token.txt`
docker compose up spire-agent -d

docker exec spire-server /opt/spire/bin/spire-server entry create -parentID spiffe://accuknox.com/1/knoxvm-cluster -spiffeID spiffe://accuknox.com/1/knoxvm-cluster/shared-informer-agent -selector docker:label:com.accuknox.com:shared-informer-agent
COMMENT

#docker compose down
