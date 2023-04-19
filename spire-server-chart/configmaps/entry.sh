#!/bin/sh

cluster_onboard_id="spiffe://accuknox.com/saas/cluster-onboarding"
knox_gateway_id="spiffe://accuknox.com/saas/knox-gateway"
policyprovider_id="spiffe://accuknox.com/saas/policyprovider-service"

SOCKET_FILE="/run/spire-server/api.sock"

cb=$(/bin/spire-server entry show -spiffeID $cluster_onboard_id -socketPath $SOCKET_FILE)
kg=$(/bin/spire-server entry show -spiffeID $knox_gateway_id -socketPath $SOCKET_FILE)
pp=$(/bin/spire-server entry show -spiffeID $policyprovider_id -socketPath $SOCKET_FILE)

if [[ -S "$SOCKET_FILE" ]]; then
     if echo "$cb" | grep -q "Entry ID"; then
          echo "Entry ID for $cluster_onboard_id already exists"
     else 
          /bin/spire-server entry create -parentID spiffe://accuknox.com/saas/agent -spiffeID $cluster_onboard_id -selector k8s_sat:sa:cluster-onboard -selector k8s_sat:pod-label:app:cluster-onboarding-service -socketPath $SOCKET_FILE
     fi
     if echo "$kg" | grep -q "Entry ID"; then
          echo "Entry ID for $knox_gateway_id already exists"
     else 
          /bin/spire-server entry create -parentID spiffe://accuknox.com/saas/agent -spiffeID $knox_gateway_id -selector k8s_sat:pod-label:app:knox-gateway -selector k8s_sat:pod-label:environment:dev -selector k8s_sat:sa:knox-gateway -socketPath $SOCKET_FILE
     fi 
     if echo "$pp" | grep -q "Entry ID"; then
          echo "Entry ID for $policyprovider_id already exists"
     else
          /bin/spire-server entry create -parentID spiffe://accuknox.com/saas/agent -spiffeID $policyprovider_id -selector k8s_sat:pod-label:app:policy-provider-service -selector k8s_sat:pod-label:environment:dev -selector k8s_sat:sa:policyprovider-service -socketPath $SOCKET_FILE
     fi
fi