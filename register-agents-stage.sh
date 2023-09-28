#!/usr/bin/env bash

if [[ $CLUSTER_ID == "" ]]
then
	echo "ERROR: \$CLUSTER_ID empty"
	exit
fi

kubectl exec -it -n accuknox-stage-spire spire-server-0 -- /opt/spire/bin/spire-server entry create -parentID spiffe://accuknox.com/1258/$CLUSTER_ID -spiffeID spiffe://accuknox.com/1258/$CLUSTER_ID/sia -selector unix:uid:$(id -u) -selector unix:gid:$(id -g) -selector unix:path:"/usr/bin/sia"

kubectl exec -it -n accuknox-stage-spire spire-server-0 -- /opt/spire/bin/spire-server entry create -parentID spiffe://accuknox.com/1258/$CLUSTER_ID -spiffeID spiffe://accuknox.com/1258/$CLUSTER_ID/fs -selector unix:uid:$(id -u) -selector unix:gid:$(id -g) -selector unix:path:"/home/feederservice/bin/feeder-service"

kubectl exec -it -n accuknox-stage-spire spire-server-0 -- /opt/spire/bin/spire-server entry create -parentID spiffe://accuknox.com/1258/$CLUSTER_ID -spiffeID spiffe://accuknox.com/1258/$CLUSTER_ID/pea -selector unix:uid:$(id -u) -selector unix:gid:$(id -g) -selector unix:path:"/home/pea/main"
