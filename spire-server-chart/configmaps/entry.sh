#!/bin/sh

SOCKET_FILE="/run/spire-server/private/api.sock"

if [[ -S "$SOCKET_FILE" ]]; then

   /bin/spire-server entry create                                       \
        -parentID spiffe://accuknox.com/saas/agent                      \
        -spiffeID spiffe://accuknox.com/saas/cluster-onboarding         \
        -selector k8s_sat:sa:cluster-onboard                            \
        -selector k8s_sat:pod-label:app:cluster-onboarding-service      \
        -socketPath $SOCKET_FILE                                    &&  \
   /bin/spire-server entry create                                       \
        -parentID spiffe://accuknox.com/saas/agent                      \
        -spiffeID spiffe://accuknox.com/saas/knox-gateway               \
        -selector k8s_sat:pod-label:app:knox-gateway                    \
        -selector k8s_sat:pod-label:environment:dev                     \
        -selector k8s_sat:sa:knox-gateway                               \
        -socketPath $SOCKET_FILE                                    &&  \
   /bin/spire-server entry create                                       \
        -parentID spiffe://accuknox.com/saas/agent                      \
        -spiffeID spiffe://accuknox.com/saas/policyprovider-service     \
        -selector k8s_sat:pod-label:app:policy-provider-service         \
        -selector k8s_sat:pod-label:environment:dev                     \
        -selector k8s_sat:sa:policyprovider-service                     \
        -socketPath $SOCKET_FILE                                    &&  \
   rm -rf /bin/spire-server

fi